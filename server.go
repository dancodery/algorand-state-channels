package main

import (
	"fmt"
	"log"
	"net"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/dancodery/algorand-state-channels/payment/testing"
)

type server struct {
	// started int32
	algod_client *algod.Client
	algo_account crypto.Account

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
			go handleConnection(conn)
		}
	}()
	return nil
}

func handleConnection(conn net.Conn) {
	fmt.Println("handleConnection new")
}

// func (s *server) stop() error {
// 	fmt.Println("stop")
// 	return nil
// }

// func (s *server) queryHandler() {

// }

// signalChan := make(chan os.Signal, 1)
// signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

// func (c *config) init(args []string) error {
// 	return nil
// }

// func run(ctx context.Context, cfg *config, out io.Writer) error {
// 	c.init(os.Args)
// 	log.SetOutput(out)

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return nil
// 		case <-time.After(1 * time.Second):
// 			log.Println("tick")
// 		}
// 	}
// }

// go func() {
// 	for {
// 		select {
// 		case s:= <-signalChan:
// 			switch s {
// 			case syscall.SIGINT, syscall.SIGTERM:
// 				log.Printf("Received SIGINT/SIGTERM, exiting gracefully...")
// 				cancel()
// 				os.Exit(1)
// 			case syscall.SIGHUP:
// 				log.Printf("Received SIGHUP, reloading...")
// 				c.init(os.Args)
// 			}
// 		case <-ctx.Done():
// 			log.Printf("Done.")
// 			os.Exit(1)
// 		}
// 	}
// }()
