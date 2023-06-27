package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	DEFAULT_GRPC_PORT = 50051
)

func getClient(ctx *cli.Context) asrpc.ASRPCClient {
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	conn, err := grpc.Dial("localhost:"+strconv.Itoa(DEFAULT_GRPC_PORT), opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	return asrpc.NewASRPCClient(conn)
}

func printJson(v interface{}) {
	jsonData, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonData))
}

func main() {
	app := cli.NewApp()
	app.Name = "ascli"
	app.Usage = "control plane for asd"
	app.Commands = []cli.Command{
		getInfoCommand,
		openChannelCommand,
		payCommand,
		closeChannelCommand,
		initiateChannelClosingCommand,
		finalizeChannelClosingCommand,
		// cooperativecloseChannelCommand,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
