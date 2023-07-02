package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/mnemonic"
	"github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/dancodery/algorand-state-channels/payment"
	"github.com/dancodery/algorand-state-channels/payment/testing"
)

type paymentChannelInfo struct {
	app_id     uint64
	partner_ip string

	alice_address string
	bob_address   string

	alice_onchain_balance uint64
	bob_onchain_balance   uint64

	total_deposit   uint64
	penalty_reserve uint64
	dispute_window  uint64
}

type paymentChannelOffChainState struct {
	timestamp int64 // unix timestamp in nanoseconds

	alice_balance uint64
	bob_balance   uint64

	alice_signature []byte
	bob_signature   []byte

	algorand_port int
	app_id        uint64
}

type server struct {
	algod_client *algod.Client
	algo_account crypto.Account

	payment_channels_onchain_states      map[string]paymentChannelInfo
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

		payment_channels_onchain_states:      make(map[string]paymentChannelInfo),
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

	// Get the IP address of the connection partner
	partner_ip := conn.RemoteAddr().(*net.TCPAddr).IP.String()

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
		s.savePaymentChannelOnChainState(partner_ip, app_id, blockchain_app_info.Params.GlobalState)

		fmt.Printf("\nThe payment channel with app_id %d was opened successfully.\n", app_id)

		fmt.Printf("All Current Payment Channels: %+v\n\n", s.payment_channels_onchain_states)

		s.UpdateWatchtowerState()

		server_response.Message = "approve"

	case "pay_request":
		counterparty_address := string(client_request.Args[0])
		alice_new_balance := binary.BigEndian.Uint64(client_request.Args[1])
		bob_new_balance := binary.BigEndian.Uint64(client_request.Args[2])
		new_timestamp := int64(binary.BigEndian.Uint64(client_request.Args[3]))

		channel_partner_signature := client_request.Args[4]

		// 1. load onchain state
		onchain_state, ok := s.payment_channels_onchain_states[counterparty_address]
		if !ok {
			fmt.Printf("Error: payment channel with address %s does not exist\n", counterparty_address)
			server_response.Message = "reject"
			break
		}

		var me_alice bool
		if onchain_state.alice_address == s.algo_account.Address.String() {
			me_alice = true
		} else {
			me_alice = false
		}

		// 2. load latest off chain state
		payment_log, ok := s.payment_channels_offchain_states_log[counterparty_address]
		if !ok {
			fmt.Printf("Error: payment channel with address %s does not exist\n", counterparty_address)
			server_response.Message = "reject"
			break
		}
		latestOffChainState, err := getLatestOffChainState(payment_log)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			server_response.Message = "reject"
			break
		}
		var last_my_balance uint64
		var last_counterparty_balance uint64
		var my_new_balance uint64
		var counterparty_new_balance uint64
		if me_alice {
			last_my_balance = latestOffChainState.alice_balance
			last_counterparty_balance = latestOffChainState.bob_balance
			my_new_balance = alice_new_balance
			counterparty_new_balance = bob_new_balance
		} else {
			last_my_balance = latestOffChainState.bob_balance
			last_counterparty_balance = latestOffChainState.alice_balance
			my_new_balance = bob_new_balance
			counterparty_new_balance = alice_new_balance
		}
		last_timestamp := latestOffChainState.timestamp

		// 3. verify that all new parameters are beneficial for me
		counterparty_balance_diff := int64(last_counterparty_balance) - int64(counterparty_new_balance)
		my_balance_diff := int64(last_my_balance) - int64(my_new_balance)

		if !(counterparty_balance_diff > 0 && // counterparty must pay to us
			my_balance_diff == (-1)*counterparty_balance_diff && // what bob gains, alice loses
			counterparty_new_balance >= onchain_state.penalty_reserve && // alice must have enough funds to pay the penalty
			last_timestamp < new_timestamp) { // timestamp must be increasing

			fmt.Println("Error: invalid new balances")
			server_response.Message = "reject"
			break
		}

		// 4. verify channel partner signature
		channel_partner_signature_correct := payment.VerifyState(
			onchain_state.app_id,
			alice_new_balance,
			bob_new_balance,
			4161,
			channel_partner_signature,
			counterparty_address,
			new_timestamp,
		)
		if !channel_partner_signature_correct {
			fmt.Println("Error: invalid channel partner signature")
			server_response.Message = "reject"
			break
		}

		// 5. sign the state as well
		my_signature, err := payment.SignState(
			onchain_state.app_id,
			s.algo_account,
			alice_new_balance,
			bob_new_balance,
			4161,
			new_timestamp,
		)
		if err != nil {
			log.Fatalf("Error signing state: %v\n", err)
			return
		}

		var alice_signature []byte
		var bob_signature []byte
		if me_alice {
			alice_signature = my_signature
			bob_signature = channel_partner_signature
		} else {
			alice_signature = channel_partner_signature
			bob_signature = my_signature
		}

		// 6. save new state
		off_chain_state := &paymentChannelOffChainState{
			timestamp: new_timestamp,

			alice_balance: alice_new_balance,
			bob_balance:   bob_new_balance,

			alice_signature: alice_signature,
			bob_signature:   bob_signature,

			algorand_port: 4161,
			app_id:        onchain_state.app_id,
		}

		if s.payment_channels_offchain_states_log[counterparty_address] == nil {
			s.payment_channels_offchain_states_log[counterparty_address] = make(map[int64]paymentChannelOffChainState)
		}
		s.payment_channels_offchain_states_log[counterparty_address][new_timestamp] = *off_chain_state

		// 7. send response to client
		server_response.Message = "approve"
		server_response.Data = [][]byte{
			my_signature,
		}

		fmt.Printf("Process payment_request of %d microalgos\n", counterparty_balance_diff)
		fmt.Printf("Alice new balance: %d\n", alice_new_balance)
		fmt.Printf("Bob new balance: %d\n\n", bob_new_balance)

	case "close_channel_request":
		counterparty_address := string(client_request.Args[0])
		channel_partner_signature := client_request.Args[1]

		// 1. load onchain state
		onchain_state, ok := s.payment_channels_onchain_states[counterparty_address]
		if !ok {
			fmt.Printf("Error: payment channel with address %s does not exist\n", counterparty_address)
			server_response.Message = "reject"
			break
		}

		// 2. load latest off chain state
		latestOffChainState, err := getLatestOffChainState(s.payment_channels_offchain_states_log[counterparty_address])
		if err != nil {
			fmt.Printf("Error getting latest off chain state: %v\n", err)
			server_response.Message = "reject"
			break
		}

		// 3. verify channel partner signature
		channel_partner_signature_correct := payment.VerifyClose(
			onchain_state.app_id,
			latestOffChainState.alice_balance,
			latestOffChainState.bob_balance,
			4161,
			channel_partner_signature,
			counterparty_address,
			latestOffChainState.timestamp,
		)
		if !channel_partner_signature_correct {
			fmt.Println("Error: invalid channel partner signature")
			server_response.Message = "reject"
			break
		}

		// 4. sign the state as well
		my_signature, err := payment.SignClose(
			onchain_state.app_id,
			s.algo_account,
			latestOffChainState.alice_balance,
			latestOffChainState.bob_balance,
			4161,
			latestOffChainState.timestamp,
		)
		if err != nil {
			log.Fatalf("Error signing state: %v\n", err)
			return
		}

		// 5. send response to client
		server_response.Message = "approve"
		server_response.Data = [][]byte{
			my_signature,
		}

		fmt.Printf("Processed close_channel_request with app_id %d\n", onchain_state.app_id)
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

func getLatestOffChainState(payment_log map[int64]paymentChannelOffChainState) (*paymentChannelOffChainState, error) {
	latest_timestamp := int64(0)
	var latest_offchain_state paymentChannelOffChainState

	for timestamp, offchain_state := range payment_log {
		if timestamp > latest_timestamp {
			latest_timestamp = timestamp
			latest_offchain_state = offchain_state
		}
	}

	if latest_timestamp == 0 {
		return nil, errors.New("no off chain state found")
	}

	return &latest_offchain_state, nil
}

func getHighestBalanceOffChainState(is_alice bool, payment_log map[int64]paymentChannelOffChainState) (*paymentChannelOffChainState, error) {
	highest_balance := uint64(0)
	var highest_balance_offchain_state paymentChannelOffChainState

	for _, offchain_state := range payment_log {
		var balance uint64
		if is_alice {
			balance = offchain_state.alice_balance
		} else {
			balance = offchain_state.bob_balance
		}

		if balance > highest_balance {
			highest_balance = balance
			highest_balance_offchain_state = offchain_state
		}
	}

	if highest_balance == 0 {
		return nil, errors.New("no off chain state found")
	}

	return &highest_balance_offchain_state, nil
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
	min_dispute_window := 2
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
	min_penalty_reserve := 100
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

func (s *server) savePaymentChannelOnChainState(partner_ip string, appID uint64, global_state []models.TealKeyValue) {
	onchain_state := &paymentChannelInfo{
		partner_ip: partner_ip,
		app_id:     appID,
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
				onchain_state.alice_onchain_balance = teal_key_value.Value.Uint
			case "latest_bob_balance":
				onchain_state.bob_onchain_balance = teal_key_value.Value.Uint
			}
		}
	}
	// save onchain_state in map
	s.payment_channels_onchain_states[onchain_state.alice_address] = *onchain_state

	// save offchain_state in log
	off_chain_state := &paymentChannelOffChainState{
		timestamp: time.Now().UnixNano(),

		alice_balance: onchain_state.alice_onchain_balance,
		bob_balance:   onchain_state.bob_onchain_balance,

		algorand_port: 4161,
		app_id:        onchain_state.app_id,
	}

	if s.payment_channels_offchain_states_log[onchain_state.alice_address] == nil {
		s.payment_channels_offchain_states_log[onchain_state.alice_address] = make(map[int64]paymentChannelOffChainState)
	}
	s.payment_channels_offchain_states_log[onchain_state.alice_address][off_chain_state.timestamp] = *off_chain_state
}

func (s *server) getAlgoBalance(address string) (uint64, error) {
	// get balance
	account_info, err := s.algod_client.AccountInformation(address).Do(context.Background())
	if err != nil {
		return 0, err
	}
	return account_info.Amount, nil
}
