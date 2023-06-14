package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

func loadMain(out io.Writer) error {
	log.SetOutput(out)

	loadedConfig, err := loadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v\n", err)
		return err
	}

	// start asd server
	server, err := newServer(loadedConfig.PeerPort)
	if err != nil {
		log.Fatalf("failed to create server: %v\n", err)
		return err
	}
	if err := server.start(); err != nil {
		log.Fatalf("failed to start server: %v\n", err)
		return err
	}

	//	start grpc server
	// 	var opts []grpc.ServerOption
	// 	grpcServer := grpc.NewServer()

	// 	ctx := context.Background()
	// 	ctx, cancel := context.WithCancel(ctx)

	// 	defer cancel()

	// }

	return nil
}

func main() {
	if err := loadMain(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
