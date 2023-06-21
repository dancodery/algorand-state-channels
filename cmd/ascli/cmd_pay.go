package main

import (
	"context"
	"fmt"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
)

var payCommand = cli.Command{
	Name:  "pay",
	Usage: "pay the channel partner",
	Description: `
		Pay the channel partner.
		TODO: add more description here
	`,
	ArgsUsage: "amount",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "partner_ip",
			Usage: "ip address of the partner node",
		},
		cli.StringFlag{
			Name:  "partner_address",
			Usage: "address of the partner node",
		},
		cli.StringFlag{
			Name:  "amount",
			Usage: "amount to pay the channel partner",
		},
	},
	Action: pay,
}

func pay(ctx *cli.Context) error {
	if ctx.NArg() != 0 {
		return cli.NewExitError("incorrect number of arguments", 1)
	}
	if ctx.String("partner_ip") == "" {
		return cli.NewExitError("partner ip address is required", 1)
	}
	if ctx.String("partner_address") == "" {
		return cli.NewExitError("partner address is required", 1)
	}
	if ctx.String("amount") == "" {
		return cli.NewExitError("amount is required", 1)
	}

	nodeAddress := &asrpc.StateChannelNodeAddress{
		Host:        ctx.String("partner_ip"),
		AlgoAddress: ctx.String("partner_address"),
	}

	payRequest := &asrpc.PayRequest{
		PartnerNode: nodeAddress,
		Amount:      ctx.Uint64("amount"),
	}

	ctxb := context.Background()
	client := getClient(ctx)

	payResponse, err := client.Pay(ctxb, payRequest)
	if err != nil {
		return err
	}

	fmt.Println(payResponse)

	return nil
}
