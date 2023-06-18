package main

import (
	"fmt"

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
			Name:  "amount",
			Usage: "amount to pay the channel partner",
		},
	},
	Action: pay,
}

func pay(ctx *cli.Context) error {
	fmt.Println("pay called")

	return nil
}
