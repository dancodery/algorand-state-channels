package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"google.golang.org/grpc"
)

func loadMain(out io.Writer) error {
	log.SetOutput(out)

	// load server config
	loadedConfig, err := loadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v\n", err)
		return err
	}

	// initialize server environment
	server, err := initializeServer(loadedConfig.PeerPort, loadedConfig.GRPCPort)
	if err != nil {
		log.Fatalf("failed to create server: %v\n", err)
		return err
	}
	if err := server.startListening(); err != nil {
		log.Fatalf("failed to start server: %v\n", err)
	}

	// start grpc server
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	asrpc.RegisterASRPCServer(grpcServer, server.rpcServer)
	fmt.Printf("Started grpc server on port %d\n", loadedConfig.GRPCPort)

	if err := grpcServer.Serve(server.grpc_listener); err != nil {
		log.Fatalf("failed to serve: %v\n", err)
		return err
	}

	return nil
}

func main() {
	if err := loadMain(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
