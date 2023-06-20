package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/dancodery/algorand-state-channels/payment/testing"
)

type paymentChannelState struct {
	app_id uint64

	timestamp uint64
}

type server struct {
	// started int32
	algod_client *algod.Client
	algo_account crypto.Account

	payment_channel_app_ids   []uint64
	payment_channel_state_log map[uint64]paymentChannelState

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
	}

	s.rpcServer = newRpcServer(s)

	s.algod_client = testing.GetAlgodClient()
	s.algo_account = crypto.GenerateAccount()

	fmt.Printf("Node ALGO address: %v\n", s.algo_account.Address.String())

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
	case "open_channel":
		app_id := parseInt(client_request.Args[0])
		s.payment_channel_app_ids = append(s.payment_channel_app_ids, uint64(app_id))

		// print payment channel app ids
		fmt.Printf("Payment channel app ids: %v\n", s.payment_channel_app_ids)

		fmt.Printf("I was notified that payment channel with app_id %d was opened\n", app_id)
		server_response.Message = "approve"
	case "close_channel":
		fmt.Println("close_channel")
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

// func (s *server) stop() error {
// 	fmt.Println("stop")
// 	return nil
// }
