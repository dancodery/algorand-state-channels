package main

import (
	"context"
	"fmt"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
)

var getInfoCommand = cli.Command{
	Name: "getinfo",
	Description: `
		Get information about the node.
		TODO: add more description here
	`,
	Usage: "getinfo --node=<node>",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "node",
			Usage: "node to connect to",
		},
	},
	Action: getInfo,
}

func getInfo(ctx *cli.Context) error {
	if ctx.String("node") == "" {
		ctx.Set("node", "localhost")
	}

	ctxb := context.Background()
	client := getClient(ctx, ctx.String("node"))

	getInfoRequest := &asrpc.GetInfoRequest{}

	getInfoResponse, err := client.GetInfo(ctxb, getInfoRequest)

	if err != nil {
		return err
	}

	fmt.Println(getInfoResponse)

	return nil
}
