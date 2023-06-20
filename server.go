package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/dancodery/algorand-state-channels/payment/testing"
)

type paymentChannelState struct {
	app_id uint64

	alice_latest_balance uint64
	bob_latest_balance   uint64

	timestamp uint64
}

type server struct {
	// started int32
	algod_client *algod.Client
	algo_account crypto.Account

	payment_channel_app_ids         []uint64
	payment_channel_state_of_app_id map[uint64]paymentChannelState
	payment_channel_onchain_state   map[string]paymentChannelState

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

		// payment_channel_app_ids:         make([]uint64, 0),
		payment_channel_state_of_app_id: make(map[uint64]paymentChannelState),
	}

	s.rpcServer = newRpcServer(s)

	s.algod_client = testing.GetAlgodClient()
	s.algo_account = crypto.GenerateAccount()

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

	client_request_data := make([]byte, 1024)
	n, err := conn.Read(client_request_data)
	if err != nil {
		log.Fatalf("Error reading: %v\n", err)
		return
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
		app_id := uint64(parseInt(client_request.Args[0]))

		// read smart contract from the blockchain for given app_id
		app_info, err := s.algod_client.GetApplicationByID(app_id).Do(context.Background())
		if err != nil {
			log.Fatalf("Error reading smart contract from blockchain: %v\n", err)
			return
		}

		// TODO: verify smart contract hash with local copy

		// savePaymentChannelState(app_info.Params.GlobalState, &s.payment_channel_state_of_app_id[app_id])
		printGlobalSmartContractState(app_info.Params.GlobalState)

		s.payment_channel_app_ids = append(s.payment_channel_app_ids, uint64(app_id))
		fmt.Printf("I was notified that payment channel with app_id %d was opened\n", app_id)
		server_response.Message = "approve"
	case "close_channel":
		fmt.Println("close_channel")
	case "pay":
		fmt.Println("received payment")
		fmt.Printf("I received %d ALGO from my partner\n", parseInt(client_request.Args[0]))
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

func printGlobalSmartContractState(global_state []models.TealKeyValue) {
	for _, teal_key_value := range global_state {
		// decode base64 for teal_key_value.Key
		decoded_key, err := base64.StdEncoding.DecodeString(teal_key_value.Key)
		if err != nil {
			log.Fatalf("Error decoding base64: %v\n", err)
		}
		fmt.Printf("Decoded Key: %s", decoded_key)

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
			fmt.Printf("Value: %s\n", address)

			// fmt.Printf("Decoded Value: %s\n", decoded_value)
		case 2: // it's uint64
			fmt.Printf("Value: %d\n", teal_key_value.Value.Uint)
		}
	}
}

// func savePaymentChannelState(global_state []types.TealKeyValue, state *paymentChannelState) {
// 	for teal_key, teal_value := range global_state {
// 		switch teal_key {
// 		case "alice_latest_balance":
// 			state.alice_latest_balance = uint64(parseInt(teal_value.Value))
// 		case "bob_latest_balance":
// 			state.bob_latest_balance = uint64(parseInt(teal_value.Value))
// 		case "timestamp":
// 			state.timestamp = uint64(parseInt(teal_value.Value))
// 		}
// 	}
// }

// func (s *server) stop() error {
// 	fmt.Println("stop")
// 	return nil
// }
