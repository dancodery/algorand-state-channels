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
		Pay the channel partner off-chain.
		Only the algo address of the partner and the amount to pay are required.
	`,
	ArgsUsage: "amount",
	Flags: []cli.Flag{
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
	if ctx.String("partner_address") == "" {
		return cli.NewExitError("partner address is required", 1)
	}
	if ctx.String("amount") == "" {
		return cli.NewExitError("amount is required", 1)
	}

	payRequest := &asrpc.PayRequest{
		AlgoAddress: ctx.String("partner_address"),
		Amount:      ctx.Uint64("amount"),
	}

	ctxb := context.Background()
	client := getClient(ctx)

	payResponse, err := client.Pay(ctxb, payRequest)
	if err != nil {
		return err
	}

	fmt.Println("Response from gRPC server: ", payResponse)

	return nil
}
