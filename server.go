package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/mnemonic"
	"github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/dancodery/algorand-state-channels/payment"
	"github.com/dancodery/algorand-state-channels/payment/testing"
)

type paymentChannelOnChainState struct {
	app_id uint64

	alice_address string
	bob_address   string

	alice_latest_balance uint64
	bob_latest_balance   uint64

	total_deposit   uint64
	penalty_reserve uint64
	dispute_window  uint64
}

type paymentChannelOffChainState struct {
	timestamp int64

	alice_balance uint64
	bob_balance   uint64

	alice_signature []byte
	bob_signature   []byte

	algorand_port int
	app_id        uint64
}

type server struct {
	// started int32
	algod_client *algod.Client
	algo_account crypto.Account

	// payment_channel_app_ids              []uint64
	// payment_channel_state_of_app_id      map[uint64]paymentChannelOnChainState
	payment_channels_onchain_states      map[string]paymentChannelOnChainState
	payment_channels_offchain_states_log map[string]map[int64]paymentChannelOffChainState

	peer_port     int
	grpc_port     int
	peer_listener net.Listener
	grpc_listener net.Listener
	rpcServer     *rpcServer
}

func initializeServer(peerPort int, grpcPort int) (*server, error) {
	s := &server{
		peer_port: peerPort,
		grpc_port: grpcPort,

		payment_channels_onchain_states:      make(map[string]paymentChannelOnChainState),
		payment_channels_offchain_states_log: make(map[string]map[int64]paymentChannelOffChainState),
	}

	s.rpcServer = newRpcServer(s)
	s.algod_client = testing.GetAlgodClient()

	// new: generate account from seed
	seed_phrase := os.Getenv("SEED_PHRASE")

	if seed_phrase == "" {
		s.algo_account = crypto.GenerateAccount()
	} else {
		private_key, err := mnemonic.ToPrivateKey(seed_phrase)
		if err != nil {
			log.Fatalf("failed to generate account from seed: %v\n", err)
			return nil, err
		}
		s.algo_account, err = crypto.AccountFromPrivateKey(private_key)
		if err != nil {
			log.Fatalf("failed to generate account from seed: %v\n", err)
			return nil, err
		}
	}

	fmt.Printf("My node ALGO address is: %v\n", s.algo_account.Address.String())
	fmt.Printf("My Public key: %v\n", s.algo_account.PublicKey)

	// fund account
	testing.FundAccount(s.algod_client, s.algo_account.Address.String(), 10_000_000_000)

	return s, nil
}

func (s *server) startListening() error {
	// save listeners
	peer_listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.peer_port))
	if err != nil {
		log.Fatalf("Error listening: %v\n", err)
		return err
	}
	s.peer_listener = peer_listener

	grpc_listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.grpc_port))
	if err != nil {
		log.Fatalf("Error listening: %v\n", err)
		return err
	}
	s.grpc_listener = grpc_listener

	// start listening
	go func() {
		fmt.Printf("Listening for peers on port %d\n", s.peer_port)
		for {
			conn, err := s.peer_listener.Accept()
			if err != nil {
				log.Fatalf("Error accepting: %v\n", err)
				return
			}
			go s.handleConnection(conn)
		}
	}()
	return nil
}

func (s *server) handleConnection(conn net.Conn) {
	defer conn.Close()

	client_request_data := make([]byte, 2<<10) // 2KB
	n, err := conn.Read(client_request_data)
	if err != nil {
		log.Fatalf("Error reading: %v\n", err)
		return // handles next connection
	}

	var client_request P2PRequest
	err = json.Unmarshal(client_request_data[:n], &client_request)
	if err != nil {
		log.Fatalf("Error unmarshalling: %v\n", err)
		return
	}

	// process request
	var server_response P2PResponse
	switch client_request.Command {
	case "open_channel_request":
		app_id, err := strconv.ParseUint(string(client_request.Args[0]), 10, 64)
		if err != nil {
			log.Fatalf("Error parsing app_id: %v\n", err)
			return
		}

		// read smart contract from the blockchain for given app_id
		blockchain_app_info, err := s.algod_client.GetApplicationByID(app_id).Do(context.Background())
		if err != nil {
			log.Fatalf("Error reading smart contract from blockchain: %v\n", err)
			return
		}

		// check if smart contract is valid
		smart_contract_valid := s.doOpenChannelSecurityChecks(blockchain_app_info)
		if !smart_contract_valid {
			server_response.Message = "reject"
			break
		}

		// save the new payment channel state
		s.savePaymentChannelOnChainState(app_id, blockchain_app_info.Params.GlobalState)

		fmt.Printf("The payment channel with app_id %d was opened successfully.\n", app_id)

		// print s.payment_channels_onchain_states
		fmt.Println("payment_channels_onchain_states: ", s.payment_channels_onchain_states)
		server_response.Message = "approve"

	case "pay_request":
		alice_address := string(client_request.Args[0])
		alice_new_balance := binary.BigEndian.Uint64(client_request.Args[1])
		bob_new_balance := binary.BigEndian.Uint64(client_request.Args[2])

		fmt.Println("Received alice signature: ", client_request.Args[3])
		channel_partner_signature := client_request.Args[3]

		// 1. load latest state
		onchain_state, ok := s.payment_channels_onchain_states[alice_address]
		if !ok {
			fmt.Printf("Error: payment channel with address %s does not exist\n", alice_address)
			server_response.Message = "reject"
			break
		}
		last_alice_balance := onchain_state.alice_latest_balance
		last_bob_balance := onchain_state.bob_latest_balance

		// 2. verify that all new parameters are beneficial for me
		alice_balance_diff := int64(alice_new_balance) - int64(last_alice_balance)
		bob_balance_diff := int64(bob_new_balance) - int64(last_bob_balance)

		if !(alice_balance_diff < 0 && // alice new balance must be smaller than old balance
			bob_balance_diff == (-1)*alice_balance_diff && // what bob gains, alice loses
			alice_new_balance >= onchain_state.penalty_reserve) { // alice must have enough funds to pay the penalty

			fmt.Println("Error: invalid new balances")
			server_response.Message = "reject"
			break
		}

		// 3. verify channel partner signature
		channel_partner_signature_correct := payment.VerifyState(
			onchain_state.app_id,
			alice_new_balance,
			bob_new_balance,
			4161,
			channel_partner_signature,
			alice_address,
		)
		if !channel_partner_signature_correct {
			fmt.Println("Error: invalid channel partner signature")
			server_response.Message = "reject"
			break
		}

		// 4. sign the state as well
		my_signature, err := payment.SignState(
			onchain_state.app_id,
			s.algo_account,
			alice_new_balance,
			bob_new_balance,
			4161,
		)
		if err != nil {
			log.Fatalf("Error signing state: %v\n", err)
			return
		}

		fmt.Println("My signature for the requested state: ", my_signature)

		// 5. save new state
		var timestamp int64 = 1685318789

		off_chain_state := &paymentChannelOffChainState{
			timestamp: timestamp,

			alice_balance: alice_new_balance,
			bob_balance:   bob_new_balance,

			alice_signature: channel_partner_signature,
			bob_signature:   my_signature,

			algorand_port: 4161,
			app_id:        onchain_state.app_id,
		}

		if s.payment_channels_offchain_states_log[alice_address] == nil {
			s.payment_channels_offchain_states_log[alice_address] = make(map[int64]paymentChannelOffChainState)
		}
		s.payment_channels_offchain_states_log[alice_address][timestamp] = *off_chain_state

		// 6. send response to client
		server_response.Message = "approve"
		server_response.Data = [][]byte{
			my_signature,
		}

	case "close_channel":
		fmt.Println("close_channel")
	case "pay_response":
		fmt.Println("pay_response")
	default:
		fmt.Println("Received unknown command")
	}

	// conver P2PResponse to json
	server_response_data, err := json.Marshal(server_response)
	if err != nil {
		log.Fatalf("Error marshalling: %v\n", err)
		return
	}

	// send response to client
	_, err = conn.Write(server_response_data)
	if err != nil {
		log.Fatalf("Error writing: %v\n", err)
		return
	}
}

func parseInt(s string) int {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	if err != nil {
		log.Fatalf("Error parsing int: %v\n", err)
	}
	return i
}

func (s *server) doOpenChannelSecurityChecks(blockchain_app_info models.Application) bool {
	// 1. verify smart contracts with local copy
	expected_approval_program, expected_clearstate_program := payment.CompilePaymentPrograms(s.algod_client)

	requested_approval_program := blockchain_app_info.Params.ApprovalProgram
	requested_clearstate_program := blockchain_app_info.Params.ClearStateProgram

	smart_contracs_equal := bytes.Equal(requested_approval_program, expected_approval_program) &&
		bytes.Equal(requested_clearstate_program, expected_clearstate_program)
	if !smart_contracs_equal {
		return false
	}

	// 2. verify that my address is bob_address
	my_address := s.algo_account.Address.String()
	bob_address_value := GetValueOfGlobalState(blockchain_app_info.Params.GlobalState, "bob_address")
	if bob_address_value == nil {
		fmt.Println("bob_address not found in global state")
		return false
	}
	bob_address, err := types.EncodeAddress(bob_address_value)
	if err != nil {
		log.Fatalf("Error encoding address: %v\n", err)
	}
	algo_addresses_equal := my_address == bob_address
	if !algo_addresses_equal {
		return false
	}

	// 3. verify that dispute_window is above min_dispute_window and below max_dispute_window
	dispute_window_value := GetValueOfGlobalState(blockchain_app_info.Params.GlobalState, "dispute_window")
	if dispute_window_value == nil {
		fmt.Println("dispute_window not found in global state")
		return false
	}
	dispute_window := parseInt(string(dispute_window_value))
	min_dispute_window := 500
	max_dispute_window := 10_000
	dispute_window_check := dispute_window >= min_dispute_window && dispute_window <= max_dispute_window
	if !dispute_window_check {
		return false
	}

	// 4. verify that penalty is above min_threshold and below max_threshold
	penalty_value := GetValueOfGlobalState(blockchain_app_info.Params.GlobalState, "penalty_reserve")
	if penalty_value == nil {
		fmt.Println("penalty_reserve not found in global state")
		return false
	}
	penalty_reserve := parseInt(string(penalty_value))
	min_penalty_reserve := 10_000
	max_penalty_reserve := 100_000_000
	penalty_reserve_check := penalty_reserve >= min_penalty_reserve && penalty_reserve <= max_penalty_reserve
	if !penalty_reserve_check {
		return false
	}

	return true
}

func GetValueOfGlobalState(global_state []models.TealKeyValue, key string) []byte {
	for _, teal_key_value := range global_state {
		// decode base64 for teal_key_value.Key
		decoded_key, err := base64.StdEncoding.DecodeString(teal_key_value.Key)
		if err != nil {
			log.Fatalf("Error decoding base64: %v\n", err)
		}

		if string(decoded_key) == key {
			switch teal_key_value.Value.Type {
			case 1: // it's bytes, probably an algo address
				decoded_value, err := base64.StdEncoding.DecodeString(teal_key_value.Value.Bytes)
				if err != nil {
					log.Fatalf("Error decoding base64: %v\n", err)
				}
				return decoded_value
			case 2: // it's uint64
				return []byte(strconv.FormatUint(teal_key_value.Value.Uint, 10))
			default:
				log.Fatalf("Unknown type: %v\n", teal_key_value.Value.Type)
			}
		}
	}
	return nil
}

func (s *server) savePaymentChannelOnChainState(appID uint64, global_state []models.TealKeyValue) {
	onchain_state := &paymentChannelOnChainState{
		app_id: appID,
	}

	for _, teal_key_value := range global_state {
		// decode base64 for teal_key_value.Key
		decoded_key, err := base64.StdEncoding.DecodeString(teal_key_value.Key)
		if err != nil {
			log.Fatalf("Error decoding base64: %v\n", err)
		}

		switch teal_key_value.Value.Type {
		case 1: // it's bytes, probably an algo address
			decoded_value, err := base64.StdEncoding.DecodeString(teal_key_value.Value.Bytes)
			if err != nil {
				log.Fatalf("Error decoding base64: %v\n", err)
			}
			address, err := types.EncodeAddress(decoded_value)
			if err != nil {
				log.Fatalf("Error encoding address: %v\n", err)
			}

			switch string(decoded_key) {
			case "alice_address":
				onchain_state.alice_address = address
			case "bob_address":
				onchain_state.bob_address = address
			}
		case 2: // it's uint64

			switch string(decoded_key) {
			case "dispute_window":
				onchain_state.dispute_window = teal_key_value.Value.Uint
			case "total_deposit":
				onchain_state.total_deposit = teal_key_value.Value.Uint
			case "penalty_reserve":
				onchain_state.penalty_reserve = teal_key_value.Value.Uint
			case "latest_alice_balance":
				onchain_state.alice_latest_balance = teal_key_value.Value.Uint
			case "latest_bob_balance":
				onchain_state.bob_latest_balance = teal_key_value.Value.Uint
			}
		}
	}
	// save onchain_state in map
	s.payment_channels_onchain_states[onchain_state.alice_address] = *onchain_state
}
