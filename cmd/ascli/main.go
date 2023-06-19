package main

import (
	"fmt"
	"os"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	DEFAULT_GRPC_PORT = 50051
	DEFAULT_ENDPOINT  = "localhost:" + string(DEFAULT_GRPC_PORT)
)

func getClient(ctx *cli.Context, host string) asrpc.ASRPCClient {
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	conn, err := grpc.Dial(DEFAULT_ENDPOINT, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	return asrpc.NewASRPCClient(conn)
}

func main() {
	app := cli.NewApp()
	app.Name = "ascli"
	app.Usage = "control plane for asd"
	app.Commands = []cli.Command{
		getInfoCommand,
		openChannelCommand,
		closeChannelCommand,
		payCommand,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
