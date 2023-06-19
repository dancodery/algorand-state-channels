package payment

import (
	"fmt"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
)

func CreatePaymentApp(
	algodClient *algod.Client,
	algoAccount crypto.Account,
	partnerAlgoAddress string,
	fundingAmount int64,
	penaltyReserve int64,
	disputeWindow int64) {
	fmt.Printf("Creating payment app...\n")
}
