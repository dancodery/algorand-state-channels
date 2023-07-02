package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/mnemonic"
	"github.com/dancodery/algorand-state-channels/asrpc"
	"github.com/dancodery/algorand-state-channels/payment"
	"github.com/dancodery/algorand-state-channels/payment/testing"
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

func (r *rpcServer) Reset(ctx context.Context, in *asrpc.ResetRequest) (*asrpc.ResetResponse, error) {
	timestamp_start := timestamppb.Now()

	r.server.payment_channels_onchain_states = make(map[string]paymentChannelInfo)
	r.server.payment_channels_offchain_states_log = make(map[string]map[int64]paymentChannelOffChainState)
	r.server.algod_client = testing.GetAlgodClient()

	// new: generate account from seed
	seed_phrase := os.Getenv("SEED_PHRASE")

	if seed_phrase == "" {
		r.server.algo_account = crypto.GenerateAccount()
	} else {
		private_key, err := mnemonic.ToPrivateKey(seed_phrase)
		if err != nil {
			log.Fatalf("failed to generate account from seed: %v\n", err)
			return nil, err
		}
		r.server.algo_account, err = crypto.AccountFromPrivateKey(private_key)
		if err != nil {
			log.Fatalf("failed to generate account from seed: %v\n", err)
			return nil, err
		}
	}

	fmt.Printf("\nReset executed\n")
	fmt.Printf("My node ALGO address is: %v\n", r.server.algo_account.Address.String())

	// fund account
	testing.FundAccount(r.server.algod_client, r.server.algo_account.Address.String(), 10_000_000_000)

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}
	return &asrpc.ResetResponse{
		RuntimeRecording: runtime_recording,
	}, nil
}

func (r *rpcServer) GetInfo(ctx context.Context, in *asrpc.GetInfoRequest) (*asrpc.GetInfoResponse, error) {
	timestamp_start := timestamppb.Now()

	algo_address := r.server.algo_account.Address.String()
	algo_balance, err := r.server.getAlgoBalance(algo_address)
	if err != nil {
		return nil, err
	}

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}

	return &asrpc.GetInfoResponse{
		AlgoAddress:      algo_address,
		AlgoBalance:      algo_balance,
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

	fmt.Printf("\nCreated payment channel app with app_id: %v and funding amount: %v\n", appID, in.FundingAmount)

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

		r.server.UpdateWatchtowerState()

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

	var me_alice bool
	var my_balance uint64
	var counterparty_balance uint64
	if onchain_state.alice_address == r.server.algo_account.Address.String() {
		me_alice = true
		my_balance = latestOffChainState.alice_balance
		counterparty_balance = latestOffChainState.bob_balance
	} else {
		me_alice = false
		my_balance = latestOffChainState.bob_balance
		counterparty_balance = latestOffChainState.alice_balance
	}

	// 3. calculate new balances
	new_my_balance := my_balance - in.Amount
	new_counterparty_balance := counterparty_balance + in.Amount

	var new_alice_balance uint64
	var new_bob_balance uint64
	if me_alice {
		new_alice_balance = new_my_balance
		new_bob_balance = new_counterparty_balance
	} else {
		new_alice_balance = new_counterparty_balance
		new_bob_balance = new_my_balance
	}

	// 4. sign new state
	timestamp_now := time.Now().UnixNano()

	var my_signature []byte
	my_signature, err = payment.SignState(
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
	newBobBalanceBytes := make([]byte, 8)

	binary.BigEndian.PutUint64(newAliceBalanceBytes, new_alice_balance)
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

	// check if alice or bob have not enough balance to close channel
	if latestOffChainState.alice_balance < 1000 || latestOffChainState.bob_balance < 1000 {
		fmt.Printf("Error: not enough balance to close channel\n\n")
		return nil, fmt.Errorf("not enough balance to close channel")
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

	fmt.Printf("Initiated channel closure for app_id: %v\n\n", onchain_state.app_id)

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

	var counterparty_address string
	if r.server.algo_account.Address.String() == onchain_state.alice_address {
		counterparty_address = onchain_state.bob_address
	} else {
		counterparty_address = onchain_state.alice_address
	}

	// 2. call finalize close channel
	payment.FinalizeCloseChannel(
		r.server.algod_client,
		r.server.algo_account,
		counterparty_address,
		onchain_state.app_id)
	fmt.Printf("Finalized channel closure for app_id: %v\n\n", onchain_state.app_id)

	// 3. delete on chain state
	delete(r.server.payment_channels_onchain_states, in.AlgoAddress)

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}
	return &asrpc.FinalizeCloseChannelResponse{
		RuntimeRecording: runtime_recording,
	}, nil
}

func (r *rpcServer) CooperativeCloseChannel(ctx context.Context, in *asrpc.CooperativeCloseChannelRequest) (*asrpc.CooperativeCloseChannelResponse, error) {
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

	// 3. check if alice or bob have not enough balance to close channel
	if latestOffChainState.alice_balance < 1000 || latestOffChainState.bob_balance < 1000 {
		fmt.Printf("Error: not enough balance to close channel\n\n")
		return nil, fmt.Errorf("not enough balance to close channel")
	}

	// 4. sign cooperative close state
	var my_signature []byte
	my_signature, err = payment.SignClose(
		onchain_state.app_id,
		r.server.algo_account,
		latestOffChainState.alice_balance,
		latestOffChainState.bob_balance,
		4161,
		latestOffChainState.timestamp,
	)
	if err != nil {
		fmt.Printf("Error signing state: %v\n", err)
	}

	// 5. send cooperative close request to partner node
	server_response, err := sendRequest(onchain_state.partner_ip, P2PRequest{Command: "close_channel_request", Args: [][]byte{
		[]byte(r.server.algo_account.Address.String()), // 1. my address
		my_signature, // 2. my signature
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

	partner_verified := payment.VerifyClose(
		onchain_state.app_id,
		latestOffChainState.alice_balance,
		latestOffChainState.bob_balance,
		4161,
		partner_signature,
		in.AlgoAddress,
		latestOffChainState.timestamp,
	)
	if !partner_verified {
		fmt.Printf("Error: partner node's signature is not valid\n")
		return nil, fmt.Errorf("partner node's signature is not valid")
	}

	var is_alice bool
	if onchain_state.alice_address == r.server.algo_account.Address.String() {
		is_alice = true
	} else {
		is_alice = false
	}

	var alice_signature []byte
	var bob_signature []byte
	if is_alice {
		alice_signature = my_signature
		bob_signature = partner_signature
	} else {
		alice_signature = partner_signature
		bob_signature = my_signature
	}

	// 8. call cooperative close channel
	payment.CooperativeCloseChannel(
		r.server.algod_client,
		r.server.algo_account,
		in.AlgoAddress,
		4161,
		onchain_state.app_id,
		latestOffChainState.alice_balance,
		latestOffChainState.bob_balance,
		uint64(latestOffChainState.timestamp),
		alice_signature,
		bob_signature)

	fmt.Printf("Cooperative channel closure for app_id: %v\n\n", onchain_state.app_id)

	// 9. delete payment channel from on chain state
	delete(r.server.payment_channels_onchain_states, in.AlgoAddress)

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}
	return &asrpc.CooperativeCloseChannelResponse{
		RuntimeRecording: runtime_recording,
	}, nil
}

func (r *rpcServer) TryToCheat(ctx context.Context, in *asrpc.TryToCheatRequest) (*asrpc.TryToCheatResponse, error) {
	timestamp_start := timestamppb.Now()

	// 1. get on chain state
	onchain_state, ok := r.server.payment_channels_onchain_states[in.AlgoAddress]
	if !ok {
		fmt.Printf("Error: payment channel with partner node %v does not exist\n", in.AlgoAddress)
		return nil, fmt.Errorf("payment channel with partner node %v does not exist", in.AlgoAddress)
	}

	// 2. retrieve off chain state with highest balance
	payment_log, ok := r.server.payment_channels_offchain_states_log[in.AlgoAddress]
	if !ok {
		fmt.Printf("Error: payment channel with partner node %v does not exist\n", in.AlgoAddress)
		return nil, fmt.Errorf("payment channel with partner node %v does not exist", in.AlgoAddress)
	}
	var is_alice bool
	if onchain_state.alice_address == r.server.algo_account.Address.String() {
		is_alice = true
	} else {
		is_alice = false
	}
	highesBalanceOffChainState, err := getHighestBalanceOffChainState(is_alice, payment_log)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil, err
	}

	// 3. check if alice or bob have not enough balance to close channel
	if highesBalanceOffChainState.alice_balance < 1000 || highesBalanceOffChainState.bob_balance < 1000 {
		fmt.Printf("Error: not enough balance to close channel\n\n")
		return nil, fmt.Errorf("not enough balance to close channel")
	}

	// 4. intiate close channel
	payment.InitiateCloseChannel(
		r.server.algod_client,
		r.server.algo_account,
		4161,
		onchain_state.app_id,
		highesBalanceOffChainState.alice_balance,
		highesBalanceOffChainState.bob_balance,
		uint64(highesBalanceOffChainState.timestamp),
		highesBalanceOffChainState.alice_signature,
		highesBalanceOffChainState.bob_signature)

	fmt.Printf("Try to cheat for app_id: %v\n", onchain_state.app_id)
	fmt.Printf("Alice cheating balance: %v\n", highesBalanceOffChainState.alice_balance)
	fmt.Printf("Bob cheating balance: %v\n\n", highesBalanceOffChainState.bob_balance)

	timestamp_end := timestamppb.Now()

	runtime_recording := &asrpc.RuntimeRecording{
		TimestampStart: timestamp_start,
		TimestampEnd:   timestamp_end,
	}
	return &asrpc.TryToCheatResponse{
		RuntimeRecording: runtime_recording,
	}, nil
}
