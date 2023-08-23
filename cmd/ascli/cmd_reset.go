package main

import (
	"context"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
)

var resetCommand = cli.Command{
	Name:        "reset",
	Description: "reset the node",
	Usage:       "deletes all channels and resets the node",
	Action:      reset,
}

func reset(ctx *cli.Context) error {
	ctxb := context.Background()
	client := getClient(ctx)

	resetRequest := &asrpc.ResetRequest{}

	_, err := client.Reset(ctxb, resetRequest)

	if err != nil {
		return err
	}

	// printJson(resetResponse)

	return nil
}
