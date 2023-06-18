package main

import (
	"fmt"
	"os"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func getClient(ctx *cli.Context, host string) asrpc.ASRPCClient {
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	conn, err := grpc.Dial(host+":28547", opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	return asrpc.NewASRPCClient(conn)
}

func main() {
	app := cli.NewApp()
	app.Name = "ascli"
	app.Usage = "control panel for asd"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "rpcserver",
			Value: "localhost:28547",
			Usage: "address of asd rpc server",
		},
	}
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
