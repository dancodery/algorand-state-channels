package payment

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/transaction"
	"github.com/algorand/go-algorand-sdk/v2/types"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/sha3"
)

const NUM_UINTS = 7
const NUM_BYTE_SLICES = 3

// CompileTeal compiles a teal file into binary
func CompileTeal(algodClient *algod.Client, path string) []byte {
	file_content, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading teal file: %v\n", err)
		return nil
	}

	compiled_code, err := algodClient.TealCompile(file_content).Do(context.Background())
	if err != nil {
		fmt.Printf("Error compiling teal: %v\n", err)
		return nil
	}

	bin, err := base64.StdEncoding.DecodeString(compiled_code.Result)
	if err != nil {
		fmt.Printf("Error decoding base64: %v\n", err)
		return nil
	}
	return bin
}

func CompilePaymentPrograms(algodClient *algod.Client) (approvalProgram []byte, clearProgram []byte) {
	// compile approval program
	approvalProgram = CompileTeal(algodClient, "smart_contracts/payment_approval.teal")
	if approvalProgram == nil {
		fmt.Printf("Error compiling approval program\n")
	}

	// compile clear program
	clearProgram = CompileTeal(algodClient, "smart_contracts/payment_clear_state.teal")
	if clearProgram == nil {
		fmt.Printf("Error compiling clear program\n")
	}
	return
}

// CreatePaymentApp creates a new payment channel smart contract
func CreatePaymentApp(
	algodClient *algod.Client,
	senderAccount crypto.Account,
	partnerAlgoAddress string,
	penaltyReserve uint64,
	disputeWindow uint64) uint64 {
	approvalBinary, clearBinary := CompilePaymentPrograms(algodClient)

	// create application deployment transaction
	sp, err := algodClient.SuggestedParams().Do(context.Background())
	if err != nil {
		fmt.Printf("Error getting suggested params: %v\n", err)
	}

	penaltyReserveBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(penaltyReserveBytes, penaltyReserve)

	disputeWindowBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(disputeWindowBytes, disputeWindow)

	pk, err := types.DecodeAddress(partnerAlgoAddress)
	if err != nil {
		fmt.Printf("Error decoding address: %v\n", err)
	}
	app_args := [][]byte{
		pk[:],
		penaltyReserveBytes,
		disputeWindowBytes,
	}
	paymentAppTxn, err := transaction.MakeApplicationCreateTx(
		false,                       // opt-in
		approvalBinary, clearBinary, // approval and clear programs
		types.StateSchema{NumUint: NUM_UINTS, NumByteSlice: NUM_BYTE_SLICES}, // global state schema
		types.StateSchema{NumUint: 0, NumByteSlice: 0},                       // local state schema
		app_args,              // app arguments
		nil,                   // accounts
		nil,                   // foreign apps
		nil,                   // foreign assets
		sp,                    // suggested params
		senderAccount.Address, // sender
		nil,                   // note
		types.Digest{},        // group
		[32]byte{},            // lease
		types.ZeroAddress,     // rekey to
	)
	if err != nil {
		fmt.Printf("Error creating application create transaction: %v\n", err)
	}

	// sign transaction
	txid, signed_txn, err := crypto.SignTransaction(senderAccount.PrivateKey, paymentAppTxn)
	if err != nil {
		fmt.Printf("Error signing transaction: %v\n", err)
	}

	// submit transaction
	_, err = algodClient.SendRawTransaction(signed_txn).Do(context.Background())
	if err != nil {
		fmt.Printf("Error submitting transaction: %v\n", err)
	}

	// wait for confirmation
	confirmedTxn, err := transaction.WaitForConfirmation(algodClient, txid, 4, context.Background())
	if err != nil {
		fmt.Printf("Error waiting for confirmation: %v\n", err)
	}
	return confirmedTxn.ApplicationIndex
}

// SetupPaymentApp funds the already created payment app
func SetupPaymentApp(
	algodClient *algod.Client,
	appID uint64,
	senderAccount crypto.Account,
	fundingAmount uint64) {
	appAddr := crypto.GetApplicationAddress(appID)

	// create transaction
	sp, err := algodClient.SuggestedParams().Do(context.Background())
	if err != nil {
		fmt.Printf("Error getting suggested params: %v\n", err)
	}

	fundAppTxn, err := transaction.MakePaymentTxn(
		senderAccount.Address.String(),
		appAddr.String(),
		fundingAmount,
		nil,
		"",
		sp,
	)
	if err != nil {
		fmt.Printf("Error creating payment transaction: %v\n", err)
	}
	callAppFundTxn, err := transaction.MakeApplicationNoOpTx(
		appID,
		[][]byte{[]byte("fund")},
		nil,
		nil,
		nil,
		sp,
		senderAccount.Address,
		nil,
		types.Digest{},
		[32]byte{},
		types.ZeroAddress,
	)
	if err != nil {
		fmt.Printf("Error creating application call 'fund' transaction: %v\n", err)
	}

	// compute group id
	group_id, err := crypto.ComputeGroupID([]types.Transaction{fundAppTxn, callAppFundTxn})
	if err != nil {
		fmt.Printf("Error computing group id: %v\n", err)
	}
	fundAppTxn.Group = group_id
	callAppFundTxn.Group = group_id

	// sign transactions
	_, signedFundAppTxn, err := crypto.SignTransaction(senderAccount.PrivateKey, fundAppTxn)
	if err != nil {
		fmt.Printf("Error signing transaction: %v\n", err)
	}
	_, signedCallAppFundTxn, err := crypto.SignTransaction(senderAccount.PrivateKey, callAppFundTxn)
	if err != nil {
		fmt.Printf("Error signing transaction: %v\n", err)
	}

	var signedGroup []byte
	signedGroup = append(signedGroup, signedFundAppTxn...)
	signedGroup = append(signedGroup, signedCallAppFundTxn...)

	// submit transactions
	pending_txn_id, err := algodClient.SendRawTransaction(signedGroup).Do(context.Background())
	if err != nil {
		fmt.Printf("Error submitting transaction: %v\n", err)
	}

	// wait for confirmation
	_, err = transaction.WaitForConfirmation(algodClient, pending_txn_id, 4, context.Background())
	if err != nil {
		fmt.Printf("Error waiting for confirmation: %v\n", err)
	}
}

func SignState(
	appID uint64,
	account crypto.Account,
	aliceBalance uint64,
	bobBalance uint64,
	algorandPort uint64,
	timestamp int64,
) ([]byte, error) {
	data_raw := make([]byte, 0)
	data_raw = append(data_raw, []byte("STATE_UPDATE")...)
	data_raw = append(data_raw, uint64ToBytes(algorandPort)...)
	data_raw = append(data_raw, []byte(",")...)
	data_raw = append(data_raw, uint64ToBytes(appID)...)
	data_raw = append(data_raw, []byte(",")...)
	data_raw = append(data_raw, uint64ToBytes(aliceBalance)...)
	data_raw = append(data_raw, []byte(",")...)
	data_raw = append(data_raw, uint64ToBytes(bobBalance)...)
	data_raw = append(data_raw, []byte(",")...)
	data_raw = append(data_raw, uint64ToBytes(uint64(timestamp))...)
	data_raw = append(data_raw, []byte("END_STATE_UPDATE")...)
	data_hashed := sha3.Sum256(data_raw)

	signed_bytes := ed25519.Sign(account.PrivateKey, data_hashed[:])
	if signed_bytes == nil {
		return nil, errors.New("error signing bytes")
	}
	return signed_bytes, nil
}

func VerifyState(
	appID uint64,
	aliceBalance uint64,
	bobBalance uint64,
	algorandPort uint64,
	signature []byte,
	algo_address string,
	timestamp int64,
) bool {
	data_raw := make([]byte, 0)
	data_raw = append(data_raw, []byte("STATE_UPDATE")...)
	data_raw = append(data_raw, uint64ToBytes(algorandPort)...)
	data_raw = append(data_raw, []byte(",")...)
	data_raw = append(data_raw, uint64ToBytes(appID)...)
	data_raw = append(data_raw, []byte(",")...)
	data_raw = append(data_raw, uint64ToBytes(aliceBalance)...)
	data_raw = append(data_raw, []byte(",")...)
	data_raw = append(data_raw, uint64ToBytes(bobBalance)...)
	data_raw = append(data_raw, []byte(",")...)
	data_raw = append(data_raw, uint64ToBytes(uint64(timestamp))...)
	data_raw = append(data_raw, []byte("END_STATE_UPDATE")...)
	data_hashed := sha3.Sum256(data_raw)

	decoded_address, err := types.DecodeAddress(algo_address)
	if err != nil {
		fmt.Printf("Error decoding address: %v\n", err)
	}

	pub_key := ed25519.PublicKey(decoded_address[:])
	return ed25519.Verify(pub_key, data_hashed[:], signature)
}

func InitiateCloseChannel(
	algod_client *algod.Client,
	sender_account crypto.Account,
	// for signed hash
	algorand_port uint64,
	app_id uint64,
	alice_balance uint64,
	bob_balance uint64,
	timestamp uint64,
	// END for signed hash
	alice_signature []byte,
	bob_signature []byte,
) {
	sp, err := algod_client.SuggestedParams().Do(context.Background())
	if err != nil {
		fmt.Printf("Error getting suggested params: %v\n", err)
	}

	algorandPortBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(algorandPortBytes, algorand_port)

	aliceBalanceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(aliceBalanceBytes, alice_balance)

	bobBalanceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bobBalanceBytes, bob_balance)

	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, timestamp)

	app_args := [][]byte{
		[]byte("initiateChannelClosing"),
		// BEGIN SIGNED VALUES
		algorandPortBytes, // algorand_port
		aliceBalanceBytes, // alice_balance
		bobBalanceBytes,   // bob_balance
		timestampBytes,    // timestamp
		// END SIGNED VALUES
		alice_signature,
		bob_signature,
	}
	callInitiateChannelClosingTxn, err := transaction.MakeApplicationNoOpTx(
		app_id,                 // app_id
		app_args,               // app_args
		nil,                    // accounts
		nil,                    // foreign_apps
		nil,                    // foreign_assets
		sp,                     // sp
		sender_account.Address, // sender
		nil,                    // note
		types.Digest{},         // group
		[32]byte{},             // lease
		types.ZeroAddress,      // rekey_to
	)
	if err != nil {
		fmt.Printf("Error creating application call 'initiateChannelClosing' transaction: %v\n", err)
	}

	// increase budget and send transaction
	IncreaseBudgetSignAndSendTransaction(
		algod_client,
		app_id,
		sender_account,
		callInitiateChannelClosingTxn,
		3930) // 1x Sha3_256 a 130 + 2x Ed25519Verify a 1900
}

func RaiseDispute(
	algod_client *algod.Client,
	sender_account crypto.Account,
	// for signed hash
	algorand_port uint64,
	app_id uint64,
	alice_balance uint64,
	bob_balance uint64,
	timestamp uint64,
	// END for signed hash
	alice_signature []byte,
	bob_signature []byte,
) {
	sp, err := algod_client.SuggestedParams().Do(context.Background())
	if err != nil {
		fmt.Printf("Error getting suggested params: %v\n", err)
	}

	algorandPortBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(algorandPortBytes, algorand_port)

	aliceBalanceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(aliceBalanceBytes, alice_balance)

	bobBalanceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bobBalanceBytes, bob_balance)

	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, timestamp)

	app_args := [][]byte{
		[]byte("raiseDispute"),
		// BEGIN SIGNED VALUES
		algorandPortBytes, // algorand_port
		aliceBalanceBytes, // alice_balance
		bobBalanceBytes,   // bob_balance
		timestampBytes,    // timestamp
		// END SIGNED VALUES
		alice_signature,
		bob_signature,
	}

	callRaiseDisputeTxn, err := transaction.MakeApplicationNoOpTx(
		app_id,                 // app_id
		app_args,               // app_args
		nil,                    // accounts
		nil,                    // foreign_apps
		nil,                    // foreign_assets
		sp,                     // sp
		sender_account.Address, // sender
		nil,                    // note
		types.Digest{},         // group
		[32]byte{},             // lease
		types.ZeroAddress,      // rekey_to
	)
	if err != nil {
		fmt.Printf("Error creating application call 'raiseDispute' transaction: %v\n", err)
	}

	IncreaseBudgetSignAndSendTransaction(
		algod_client,
		app_id,
		sender_account,
		callRaiseDisputeTxn,
		3930) // 1x Sha3_256 a 130 + 2x Ed25519Verify a 1900
}

func FinalizeCloseChannel(
	algod_client *algod.Client,
	sender_account crypto.Account,
	counterparty_address string,
	app_id uint64,
) {
	sp, err := algod_client.SuggestedParams().Do(context.Background())
	if err != nil {
		fmt.Printf("Error getting suggested params: %v\n", err)
	}

	app_args := [][]byte{
		[]byte("finalizeChannelClosing"),
	}
	callFinalizeChannelClosingTxn, err := transaction.MakeApplicationNoOpTx(
		app_id,   // app_id
		app_args, // app_args
		[]string{
			sender_account.Address.String(),
			counterparty_address,
		}, // accounts
		nil,                    // foreign_apps
		nil,                    // foreign_assets
		sp,                     // sp
		sender_account.Address, // sender
		nil,                    // note
		types.Digest{},         // group
		[32]byte{},             // lease
		types.ZeroAddress,      // rekey_to
	)
	if err != nil {
		fmt.Printf("Error creating application call 'finalizeChannelClosing' transaction: %v\n", err)
	}

	// sign transaction
	_, signedCallFinalizeChannelClosingTxn, err := crypto.SignTransaction(sender_account.PrivateKey, callFinalizeChannelClosingTxn)
	if err != nil {
		fmt.Printf("Failed to sign transaction: %v\n", err)
	}

	// send transaction
	pending_txn_id, err := algod_client.SendRawTransaction(signedCallFinalizeChannelClosingTxn).Do(context.Background())
	if err != nil {
		fmt.Printf("Failed to send transaction: %v\n", err)
	}

	// wait for confirmation
	_, err = transaction.WaitForConfirmation(algod_client, pending_txn_id, 4, context.Background())
	if err != nil {
		fmt.Printf("Error waiting for confirmation: %v\n", err)
	}
}

func uint64ToBytes(val uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, val)
	return b
}

func IncreaseBudgetSignAndSendTransaction(
	client *algod.Client,
	appID uint64,
	sender crypto.Account,
	unsignedMainTransaction types.Transaction,
	targetAmount uint64,
) {
	// get suggested params
	sp, err := client.SuggestedParams().Do(context.Background())
	if err != nil {
		fmt.Printf("Error getting suggested params: %v\n", err)
	}

	amountOfIncreaseBudgetTransactions := math.Ceil(float64(targetAmount) / 700)

	// create unsigned transactions
	var unsignedIncreaseBudgetTransactions []types.Transaction
	for i := 0; i < int(amountOfIncreaseBudgetTransactions); i++ {
		increaseBudgetAppTxn, err := transaction.MakeApplicationNoOpTx(
			appID, // app_id
			[][]byte{
				[]byte("increaseBudget"),
				[]byte(strconv.Itoa(i)),
			}, // app_args
			nil,               // accounts
			nil,               // foreign_apps
			nil,               // foreign_assets
			sp,                // suggested params
			sender.Address,    // sender
			nil,               // note
			types.Digest{},    // group
			[32]byte{},        // lease
			types.ZeroAddress, // rekey_to
		)
		if err != nil {
			fmt.Printf("Error creating application call 'increaseBudget' transaction: %v\n", err)
		}
		unsignedIncreaseBudgetTransactions = append(unsignedIncreaseBudgetTransactions, increaseBudgetAppTxn)
	}

	// compute group id
	group_id, err := crypto.ComputeGroupID(append([]types.Transaction{unsignedMainTransaction}, unsignedIncreaseBudgetTransactions...))
	if err != nil {
		fmt.Printf("Error computing group id: %v\n", err)
	}
	unsignedMainTransaction.Group = group_id
	for i := 0; i < len(unsignedIncreaseBudgetTransactions); i++ {
		unsignedIncreaseBudgetTransactions[i].Group = group_id
	}

	// sign transactions

	_, signedMainTransaction, err := crypto.SignTransaction(sender.PrivateKey, unsignedMainTransaction)
	if err != nil {
		fmt.Printf("Error signing main transaction: %v\n", err)
	}

	var signedIncreaseBudgetTransactions [][]byte
	// iterate over unsignedIncreaseBudgetTransactions and sign them
	for _, increaseBudgetAppTxn := range unsignedIncreaseBudgetTransactions {
		_, signedIncreaseBudgetAppTxn, err := crypto.SignTransaction(sender.PrivateKey, increaseBudgetAppTxn)
		if err != nil {
			fmt.Printf("Error signing 'increaseBudget' transaction: %v\n", err)
		}
		signedIncreaseBudgetTransactions = append(signedIncreaseBudgetTransactions, signedIncreaseBudgetAppTxn)
	}

	// append signed transactions to group
	var signedGroupTxns []byte
	signedGroupTxns = append(signedGroupTxns, signedMainTransaction...)
	for _, signedTxn := range signedIncreaseBudgetTransactions {
		signedGroupTxns = append(signedGroupTxns, signedTxn...)
	}

	// submit group transaction
	pending_txn_id, err := client.SendRawTransaction(signedGroupTxns).Do(context.Background())
	if err != nil {
		fmt.Printf("Error submitting transaction: %v\n", err)
	}

	// wait for confirmation
	_, err = transaction.WaitForConfirmation(client, pending_txn_id, 4, context.Background())
	if err != nil {
		fmt.Printf("Error waiting for confirmation: %v\n", err)
	}
}
