package main

import (
	"context"
	"fmt"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
)

var cooperativecloseChannelCommand = cli.Command{
	Name:  "cooperativeclosechannel",
	Usage: "close an existing channel",
	Description: `
		Close an existing channel with a partner cooperatively.
		Only the algo address of the partner is required.
	`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "partner_address",
			Usage: "algo address of the partner node",
		},
	},
	Action: closeChannel,
}

func closeChannel(ctx *cli.Context) error {
	if ctx.NArg() != 0 {
		return cli.NewExitError("incorrect number of arguments", 1)
	}
	if ctx.String("partner_address") == "" {
		return cli.NewExitError("partner algo address is required", 1)
	}

	close_channel_request := &asrpc.CooperativeCloseChannelRequest{
		AlgoAddress: ctx.String("partner_address"),
	}

	ctxb := context.Background()
	client := getClient(ctx)

	close_channel_response, err := client.CooperativeCloseChannel(ctxb, close_channel_request)
	if err != nil {
		return err
	}

	fmt.Println("Response from gRPC server: ", close_channel_response)

	return nil
}
