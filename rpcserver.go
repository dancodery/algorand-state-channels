package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

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

	// 2. Fund payment app
	payment.SetupPaymentApp(
		r.server.algod_client,
		appID,
		r.server.algo_account,
		in.FundingAmount)

	fmt.Printf("Created payment channel app with appID: %v and funding amount: %v\n", appID, in.FundingAmount)

	// 3. send notification to partner node
	partner_response, err := sendRequest(in.PartnerNode.Host, P2PRequest{Command: "open_channel_request", Args: [][]byte{[]byte(strconv.Itoa(int(appID)))}})
	if err != nil {
		fmt.Printf("Error sending open channel request to partner node: %v\n", err)
		return nil, err
	}

	// 4. read partner node's response
	switch partner_response.Message {
	case "approve":
		// save the payment channel on chain state
		onchain_state := &paymentChannelInfo{
			app_id:     appID,
			partner_ip: in.PartnerNode.Host,

			alice_address: r.server.algo_account.Address.String(),
			bob_address:   in.PartnerNode.AlgoAddress,

			alice_onchain_balance: in.FundingAmount,
			bob_onchain_balance:   0,

			total_deposit:   in.FundingAmount,
			penalty_reserve: in.PenaltyReserve,
			dispute_window:  in.DisputeWindow,
		}
		r.server.payment_channels_onchain_states[in.PartnerNode.AlgoAddress] = *onchain_state

		// save the payment channel off chain state
		off_chain_state := &paymentChannelOffChainState{
			timestamp: time.Now().UnixNano(),

			alice_balance: onchain_state.alice_onchain_balance,
			bob_balance:   onchain_state.bob_onchain_balance,

			algorand_port: 4161,
			app_id:        onchain_state.app_id,
		}

		if r.server.payment_channels_offchain_states_log[in.PartnerNode.AlgoAddress] == nil {
			r.server.payment_channels_offchain_states_log[in.PartnerNode.AlgoAddress] = make(map[int64]paymentChannelOffChainState)
		}
		r.server.payment_channels_offchain_states_log[in.PartnerNode.AlgoAddress][off_chain_state.timestamp] = *off_chain_state

		// print all payment channel states
		fmt.Printf("All Current Payment Channels: %+v\n\n", r.server.payment_channels_onchain_states)
	case "reject":
		fmt.Printf("Partner node rejected open channel request\n")
		return nil, fmt.Errorf("partner node rejected open channel request")
	default:
		fmt.Printf("Partner node sent invalid response to open channel request\n")
		return nil, fmt.Errorf("partner node sent invalid response to open channel request")
	}

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
	onchain_state, ok := r.server.payment_channels_onchain_states[in.AlgoAddress]
	if !ok {
		fmt.Printf("Error: payment channel with partner node %v does not exist\n", in.AlgoAddress)
		return nil, fmt.Errorf("payment channel with partner node %v does not exist", in.AlgoAddress)
	}

	// 2. retrieve old balances
	payment_log, ok := r.server.payment_channels_offchain_states_log[in.AlgoAddress]
	if !ok {
		fmt.Printf("Error: payment channel with partner node %v does not exist\n", in.AlgoAddress)
		return nil, fmt.Errorf("payment channel with partner node %v does not exist", in.AlgoAddress)
	}
	latestOffChainState, err := getLatestOffChainState(payment_log)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil, err
	}
	alice_balance := latestOffChainState.alice_balance
	bob_balance := latestOffChainState.bob_balance

	// 3. calculate new balances
	new_alice_balance := alice_balance - in.Amount
	new_bob_balance := bob_balance + in.Amount

	// 4. sign new state
	timestamp_now := time.Now().UnixNano()

	my_signature, err := payment.SignState(
		onchain_state.app_id,
		r.server.algo_account,
		new_alice_balance,
		new_bob_balance,
		4161,
		timestamp_now)
	if err != nil {
		fmt.Printf("Error signing state: %v\n", err)
	}

	// 5. send new state to partner node
	newAliceBalanceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(newAliceBalanceBytes, new_alice_balance)

	newBobBalanceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(newBobBalanceBytes, new_bob_balance)

	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, uint64(timestamp_now))

	server_response, err := sendRequest(onchain_state.partner_ip, P2PRequest{Command: "pay_request", Args: [][]byte{
		[]byte(r.server.algo_account.Address.String()), // 1. my address
		newAliceBalanceBytes,                           // 2. my new balance
		newBobBalanceBytes,                             // 3. partner's new balance
		timestampBytes,                                 // 4. timestamp
		my_signature,                                   // 5. my signature
	}})
	if err != nil {
		fmt.Printf("Error sending pay request to partner node: %v\n", err)
		return nil, err
	}

	// 6. read partner node's response
	fmt.Printf("Payment partner node's response: %v\n", server_response.Message)
	if server_response.Message != "approve" {
		fmt.Printf("Partner node rejected pay request\n")
		return nil, fmt.Errorf("partner node rejected pay request")
	}

	// 7. verify partner node's signature
	partner_signature := server_response.Data[0]

	partner_verified := payment.VerifyState(
		onchain_state.app_id,
		new_alice_balance,
		new_bob_balance,
		4161,
		partner_signature,
		in.AlgoAddress,
		timestamp_now)
	if !partner_verified {
		fmt.Printf("Partner node's signature is invalid\n")
		return nil, fmt.Errorf("partner node's signature is invalid")
	}

	// 8. save new state
	off_chain_state := &paymentChannelOffChainState{
		timestamp: timestamp_now,

		alice_balance: new_alice_balance,
		bob_balance:   new_bob_balance,

		alice_signature: my_signature,
		bob_signature:   partner_signature,

		algorand_port: 4161,
		app_id:        onchain_state.app_id,
	}

	if r.server.payment_channels_offchain_states_log[in.AlgoAddress] == nil {
		r.server.payment_channels_offchain_states_log[in.AlgoAddress] = make(map[int64]paymentChannelOffChainState)
	}
	r.server.payment_channels_offchain_states_log[in.AlgoAddress][timestamp_now] = *off_chain_state

	// 9. update on chain state
	fmt.Printf("Processed payment of %v microalgos\n", in.Amount)
	fmt.Printf("Alice new balance: %v\n", r.server.payment_channels_offchain_states_log[in.AlgoAddress][timestamp_now].alice_balance)
	fmt.Printf("Bob new balance: %v\n\n", r.server.payment_channels_offchain_states_log[in.AlgoAddress][timestamp_now].bob_balance)

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}

	return &asrpc.PayResponse{
		RuntimeRecording: runtime_recording,
	}, nil
}

func (r *rpcServer) InitiateCloseChannel(ctx context.Context, in *asrpc.InitiateCloseChannelRequest) (*asrpc.InitiateCloseChannelResponse, error) {
	timestamp_start := timestamppb.Now()

	// 1. get on chain state
	onchain_state, ok := r.server.payment_channels_onchain_states[in.AlgoAddress]
	if !ok {
		fmt.Printf("Error: payment channel with partner node %v does not exist\n", in.AlgoAddress)
		return nil, fmt.Errorf("payment channel with partner node %v does not exist", in.AlgoAddress)
	}

	// 2. retrieve latest off chain state
	payment_log, ok := r.server.payment_channels_offchain_states_log[in.AlgoAddress]
	if !ok {
		fmt.Printf("Error: payment channel with partner node %v does not exist\n", in.AlgoAddress)
		return nil, fmt.Errorf("payment channel with partner node %v does not exist", in.AlgoAddress)
	}
	latestOffChainState, err := getLatestOffChainState(payment_log)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil, err
	}

	payment.InitiateCloseChannel(
		r.server.algod_client,
		r.server.algo_account,
		4161,
		onchain_state.app_id,
		latestOffChainState.alice_balance,
		latestOffChainState.bob_balance,
		uint64(latestOffChainState.timestamp),
		latestOffChainState.alice_signature,
		latestOffChainState.bob_signature)

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}
	return &asrpc.InitiateCloseChannelResponse{
		RuntimeRecording: runtime_recording,
	}, nil
}

func (r *rpcServer) FinalizeCloseChannel(ctx context.Context, in *asrpc.FinalizeCloseChannelRequest) (*asrpc.FinalizeCloseChannelResponse, error) {
	timestamp_start := timestamppb.Now()

	// 1. get on chain state
	onchain_state, ok := r.server.payment_channels_onchain_states[in.AlgoAddress]
	if !ok {
		fmt.Printf("Error: payment channel with partner node %v does not exist\n", in.AlgoAddress)
		return nil, fmt.Errorf("payment channel with partner node %v does not exist", in.AlgoAddress)
	}

	// 2. call finalize close channel
	payment.FinalizeCloseChannel(
		r.server.algod_client,
		r.server.algo_account,
		onchain_state.app_id)

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}
	return &asrpc.FinalizeCloseChannelResponse{
		RuntimeRecording: runtime_recording,
	}, nil
}
