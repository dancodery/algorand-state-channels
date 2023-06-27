package main

import (
	"context"
	"fmt"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
)

var finalizeChannelClosingCommand = cli.Command{
	Name:  "finalizeclosechannel",
	Usage: "finalize the closing phase of an existing channel",
	Description: `
		Finalize the closing phase of an existing channel.
		`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "partner_address",
			Usage: "algo address of the partner node",
		},
	},
	Action: finalizeChannelClosing,
}

func finalizeChannelClosing(ctx *cli.Context) error {
	if ctx.NArg() != 0 {
		return cli.NewExitError("incorrect number of arguments", 1)
	}
	if ctx.String("partner_address") == "" {
		return cli.NewExitError("partner algo address is required", 1)
	}

	finalize_close_channel_request := &asrpc.FinalizeCloseChannelRequest{
		AlgoAddress: ctx.String("partner_address"),
	}

	ctxb := context.Background()
	client := getClient(ctx)

	finalize_close_channel_response, err := client.FinalizeCloseChannel(ctxb, finalize_close_channel_request)
	if err != nil {
		return err
	}

	fmt.Println(finalize_close_channel_response)

	return nil
}
