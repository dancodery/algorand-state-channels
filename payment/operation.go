package payment

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io/ioutil"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/transaction"
	"github.com/algorand/go-algorand-sdk/v2/types"
)

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

// CreatePaymentApp creates a new payment channel smart contract
func CreatePaymentApp(
	algodClient *algod.Client,
	senderAccount crypto.Account,
	partnerAlgoAddress string,
	penaltyReserve uint64,
	disputeWindow uint64) uint64 {
	// compile approval program
	approvalBinary := CompileTeal(algodClient, "smart_contracts/payment_approval.teal")
	if approvalBinary == nil {
		fmt.Printf("Error compiling approval program\n")
	}

	// compile clear program
	clearBinary := CompileTeal(algodClient, "smart_contracts/payment_clear_state.teal")
	if clearBinary == nil {
		fmt.Printf("Error compiling clear program\n")
	}

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
		types.StateSchema{NumUint: 8, NumByteSlice: 2}, // global state schema
		types.StateSchema{NumUint: 0, NumByteSlice: 0}, // local state schema
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
