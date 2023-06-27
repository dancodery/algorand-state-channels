package main

import (
	"context"
	"fmt"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
)

var initiateChannelClosingCommand = cli.Command{
	Name:  "initiateclosechannel",
	Usage: "initiate the closing phase of an existing channel",
	Description: `
		Initiate the closing phase of an existing channel.
		`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "partner_address",
			Usage: "algo address of the partner node",
		},
	},
	Action: initiateChannelClosing,
}

func initiateChannelClosing(ctx *cli.Context) error {
	if ctx.NArg() != 0 {
		return cli.NewExitError("incorrect number of arguments", 1)
	}
	if ctx.String("partner_address") == "" {
		return cli.NewExitError("partner algo address is required", 1)
	}

	initiate_close_channel_request := &asrpc.InitiateCloseChannelRequest{
		AlgoAddress: ctx.String("partner_address"),
	}

	ctxb := context.Background()
	client := getClient(ctx)

	initiate_close_channel_response, err := client.InitiateCloseChannel(ctxb, initiate_close_channel_request)
	if err != nil {
		return err
	}

	fmt.Println(initiate_close_channel_response)

	return nil
}
