package main

import (
	"context"
	"fmt"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
)

var openChannelCommand = cli.Command{
	Name:  "openchannel",
	Usage: "open a new channel to another node",
	Description: `
		Open a new channel to another node.	
		The partner's Algo address, funding amount, penalty reserve, and dispute window are required for the smart contract.
		The partner's IP address is required for the off-chain communication.
	`,
	ArgsUsage: "partner_ip funding_amount penalty_reserve dispute_window",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "partner_ip",
			Usage: "ip address of the partner node",
		},
		cli.StringFlag{
			Name:  "partner_address",
			Usage: "address of the partner node",
		},
		cli.Int64Flag{
			Name:  "funding_amount",
			Usage: "amount to fund the channel with",
		},
		cli.Int64Flag{
			Name:  "penalty_reserve",
			Usage: "amount to reserve for penalties",
		},
		cli.Int64Flag{
			Name:  "dispute_window",
			Usage: "number of blocks to wait for dispute resolution",
		},
	},
	Action: openChannel,
}

func openChannel(ctx *cli.Context) error {
	if ctx.NArg() != 0 {
		return cli.NewExitError("incorrect number of arguments", 1)
	}
	if ctx.String("partner_ip") == "" {
		return cli.NewExitError("partner ip address is required", 1)
	}
	if ctx.String("partner_address") == "" {
		return cli.NewExitError("partner address is required", 1)
	}
	if ctx.Int64("funding_amount") == 0 {
		return cli.NewExitError("funding amount is required", 1)
	}
	if ctx.Int64("penalty_reserve") == 0 {
		return cli.NewExitError("penalty reserve is required", 1)
	}
	if ctx.Int64("dispute_window") == 0 {
		return cli.NewExitError("dispute window is required", 1)
	}

	ctxb := context.Background()
	client := getClient(ctx)

	nodeAddress := &asrpc.StateChannelNodeAddress{
		Host:        ctx.String("partner_ip"),
		AlgoAddress: ctx.String("partner_address"),
	}

	openChannelRequest := &asrpc.OpenChannelRequest{
		PartnerNode:    nodeAddress,
		FundingAmount:  ctx.Uint64("funding_amount"),
		PenaltyReserve: ctx.Uint64("penalty_reserve"),
		DisputeWindow:  ctx.Uint64("dispute_window"),
	}

	openChannelResponse, err := client.OpenChannel(ctxb, openChannelRequest)
	if err != nil {
		return err
	}

	fmt.Println("Response from gRPC server: ", openChannelResponse)

	return nil
}
