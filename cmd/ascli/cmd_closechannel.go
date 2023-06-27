package main

import (
	"context"
	"fmt"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
)

var closeChannelCommand = cli.Command{
	Name:  "closechannel",
	Usage: "close an existing channel",
	Description: `
		Close an existing channel.
		TODO: add more description here
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

	close_channel_request := &asrpc.CloseChannelRequest{
		AlgoAddress: ctx.String("partner_address"),
	}

	ctxb := context.Background()
	client := getClient(ctx)

	close_channel_response, err := client.CloseChannel(ctxb, close_channel_request)
	if err != nil {
		return err
	}

	fmt.Println(close_channel_response)

	return nil
}
