package main

import (
	"context"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/urfave/cli"
)

var getInfoCommand = cli.Command{
	Name: "getinfo",
	Description: `
		Get information about the node.
		TODO: add more description here
	`,
	Usage:  "get algo address and balance of the node",
	Action: getInfo,
}

func getInfo(ctx *cli.Context) error {
	ctxb := context.Background()
	client := getClient(ctx)

	getInfoRequest := &asrpc.GetInfoRequest{}

	getInfoResponse, err := client.GetInfo(ctxb, getInfoRequest)

	if err != nil {
		return err
	}

	printJson(getInfoResponse)

	return nil
}
