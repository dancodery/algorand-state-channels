package main

import (
	"context"
	"fmt"

	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/dancodery/algorand-state-channels/payment"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type rpcServer struct {
	// started int32
	server *server

	asrpc.UnimplementedASRPCServer
}

var _ asrpc.ASRPCServer = (*rpcServer)(nil)

func newRpcServer(s *server) *rpcServer {
	return &rpcServer{
		server: s,
	}
}

func (r *rpcServer) GetInfo(ctx context.Context, in *asrpc.GetInfoRequest) (*asrpc.GetInfoResponse, error) {
	timestamp_start := timestamppb.Now()

	algo_address := r.server.algo_account.Address.String()

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}

	return &asrpc.GetInfoResponse{
		AlgoAddress:      algo_address,
		RuntimeRecording: runtime_recording,
	}, nil
}

func (r *rpcServer) OpenChannel(ctx context.Context, in *asrpc.OpenChannelRequest) (*asrpc.OpenChannelResponse, error) {
	timestamp_start := timestamppb.Now()

	// 1. Create payment app
	appID := payment.CreatePaymentApp(
		r.server.algod_client,
		r.server.algo_account,
		in.PartnerNode.AlgoAddress,
		in.PenaltyReserve,
		in.DisputeWindow)
	fmt.Printf("Created payment app with appID: %v\n", appID)

	// 2. Fund payment app
	payment.SetupPaymentApp(
		r.server.algod_client,
		appID,
		r.server.algo_account,
		in.FundingAmount)

	// 3. Notify partner node
	notifyOpenChannel(in.PartnerNode.Host, appID)

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}

	fmt.Println(in)
	return &asrpc.OpenChannelResponse{
		AppId:            appID,
		RuntimeRecording: runtime_recording,
	}, nil
}
