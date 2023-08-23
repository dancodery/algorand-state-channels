package main

import (
	"context"
	"fmt"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
)

var tryToCheatCommand = cli.Command{
	Name:  "trytocheat",
	Usage: "try to cheat (only for testing purposes)",
	Description: `

	`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "partner_address",
			Usage: "algo address of the partner node",
		},
	},
	Action: tryToCheat,
}

func tryToCheat(ctx *cli.Context) error {
	if ctx.NArg() != 0 {
		return cli.NewExitError("incorrect number of arguments", 1)
	}
	if ctx.String("partner_address") == "" {
		return cli.NewExitError("partner algo address is required", 1)
	}

	try_to_cheat_request := &asrpc.TryToCheatRequest{
		AlgoAddress: ctx.String("partner_address"),
	}

	ctxb := context.Background()
	client := getClient(ctx)

	try_to_cheat_response, err := client.TryToCheat(ctxb, try_to_cheat_request)
	if err != nil {
		return err
	}

	fmt.Println("Response from gRPC server: ", try_to_cheat_response)

	return nil
}
