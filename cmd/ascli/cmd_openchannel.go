package main

import (
	"fmt"

	"github.com/urfave/cli"
)

var openChannelCommand = cli.Command{
	Name:  "openchannel",
	Usage: "open a new channel to another node",
	Description: `
		Open a new channel to another node.	
		TODO: add more description here
	`,
	ArgsUsage: "partner_ip funding_amount penalty_reserve dispute_window",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "partner-ip",
			Usage: "ip address of the partner node",
		},
		cli.Int64Flag{
			Name:  "funding-amount",
			Usage: "amount to fund the channel with",
		},
		cli.Int64Flag{
			Name:  "penalty-reserve",
			Usage: "amount to reserve for penalties",
		},
		cli.Int64Flag{
			Name:  "dispute-window",
			Usage: "number of blocks to wait for dispute resolution",
		},
	},
	Action: openChannel,
}

func openChannel(ctx *cli.Context) error {
	fmt.Println("openChannel called")

	return nil
}
