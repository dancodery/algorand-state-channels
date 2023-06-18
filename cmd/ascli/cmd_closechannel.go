package main

import (
	"fmt"

	"github.com/urfave/cli"
)

var closeChannelCommand = cli.Command{
	Name:  "closechannel",
	Usage: "close an existing channel",
	Description: `
		Close an existing channel.
	`,
	ArgsUsage: "no arguments",
	Action:    closeChannel,
}

func closeChannel(ctx *cli.Context) error {
	fmt.Println("closeChannel called")

	return nil
}
