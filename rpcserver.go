package main

import (
	"context"
	"fmt"
	"strconv"

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
	fmt.Printf("Created payment channel app with appID: %v\n", appID)

	// 2. Fund payment app
	payment.SetupPaymentApp(
		r.server.algod_client,
		appID,
		r.server.algo_account,
		in.FundingAmount)

	// 3. send notification to partner node
	sendRequest(in.PartnerNode.Host, P2PRequest{Command: "open_channel_request", Args: []string{strconv.FormatUint(appID, 10)}})

	// 4. save payment channel on chain state
	onchain_state := &paymentChannelOnChainState{
		app_id:               appID,
		alice_address:        r.server.algo_account.Address.String(),
		bob_address:          in.PartnerNode.AlgoAddress,
		alice_latest_balance: in.FundingAmount,
		bob_latest_balance:   0,
		total_deposit:        in.FundingAmount,
		penalty_reserve:      in.PenaltyReserve,
		dispute_window:       in.DisputeWindow,
	}
	r.server.payment_channels_onchain_states[in.PartnerNode.AlgoAddress] = *onchain_state

	// print payment channel states
	fmt.Printf("Payment channel states: %v\n", r.server.payment_channels_onchain_states)

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}

	return &asrpc.OpenChannelResponse{
		AppId:            appID,
		RuntimeRecording: runtime_recording,
	}, nil
}

func (r *rpcServer) Pay(ctx context.Context, in *asrpc.PayRequest) (*asrpc.PayResponse, error) {
	timestamp_start := timestamppb.Now()

	// 1. get on chain state
	onchain_state := r.server.payment_channels_onchain_states[in.PartnerNode.AlgoAddress]

	// 1. retrieve old balances
	alice_balance := onchain_state.alice_latest_balance
	bob_balance := onchain_state.bob_latest_balance

	// 2. calculate new balances
	new_alice_balance := alice_balance - in.Amount
	new_bob_balance := bob_balance + in.Amount

	//

	// 3. sign new state
	payment.SignState(
		r.server.algod_client,
		onchain_state.app_id,
		r.server.algo_account,
		new_alice_balance,
		new_bob_balance,
		4161)

	// 3. send new state to partner node

	// send pay request to partner node
	sendRequest(in.PartnerNode.Host, P2PRequest{Command: "pay", Args: []string{strconv.FormatUint(in.Amount, 10)}})

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}

	return &asrpc.PayResponse{
		RuntimeRecording: runtime_recording,
	}, nil
}
