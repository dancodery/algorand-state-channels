package testing

import (
	"context"
	"log"

	"github.com/algorand/go-algorand-sdk/v2/client/kmd"
	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/transaction"
)

const (
	ALGOD_ADDRESS = "http://algorand-algod:4001"
	ALGOD_TOKEN   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	KMD_ADDRESS = "http://algorand-algod:4002"
	KMD_TOKEN   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	KMD_WALLET_NAME     = "unencrypted-default-wallet"
	KMD_WALLET_PASSWORD = ""
)

func GetAlgodClient() *algod.Client {
	algodClient, err := algod.MakeClient(ALGOD_ADDRESS, ALGOD_TOKEN)
	if err != nil {
		log.Fatalf("failed to make algod client: %v\n", err)
	}
	return algodClient
}

func GetKmdClient() kmd.Client {
	kmdClient, err := kmd.MakeClient(KMD_ADDRESS, KMD_TOKEN)
	if err != nil {
		log.Fatalf("Failed to create kmd client: %s", err)
	}

	return kmdClient
}

// func getGenesisAccounts() ([]*crypto.Account, error) {
// 	kmdClient := getKmdClient()

// 	response, err := kmdClient.ListWallets()

// 	// get genesis accounts
// 	algodClient := getAlgodClient()
// 	genesisAccounts, err := algodClient.GetGenesisAccounts()
// 	if err != nil {
// 		log.Fatalf("failed to get genesis accounts: %v\n", err)
// 		return nil, err
// 	}

// 	// convert genesis accounts to accounts
// 	accounts := make([]*crypto.Account, len(genesisAccounts))
// 	for i, genesisAccount := range genesisAccounts {
// 		accounts[i] = &crypto.Account{
// 			Address: genesisAccount.Address,
// 			Amount:  genesisAccount.Amount,
// 		}
// 	}

// 	return accounts, nil
// }

// func GetTemporaryAccount(algodClient *algod.Client) crypto.Account {

func GetSandboxAccounts() ([]crypto.Account, error) {
	client := GetKmdClient()

	resp, err := client.ListWallets()
	if err != nil {
		log.Fatalf("failed to list wallets: %v\n", err)
		return nil, err
	}

	var walletID string
	for _, wallet := range resp.Wallets {
		if wallet.Name == KMD_WALLET_NAME {
			walletID = wallet.ID
			break
		}
	}
	if walletID == "" {
		log.Fatalf("failed to find wallet: %v\n", err)
		return nil, err
	}

	whResp, err := client.InitWalletHandle(walletID, KMD_WALLET_PASSWORD)
	if err != nil {
		log.Fatalf("failed to init wallet handle: %v\n", err)
		return nil, err
	}

	lkResp, err := client.ListKeys(whResp.WalletHandleToken)
	if err != nil {
		log.Fatalf("failed to list keys: %v\n", err)
		return nil, err
	}

	var accounts []crypto.Account
	for _, addr := range lkResp.Addresses {
		expResp, err := client.ExportKey(whResp.WalletHandleToken, KMD_WALLET_PASSWORD, addr)
		if err != nil {
			log.Fatalf("failed to export key: %v\n", err)
			return nil, err
		}

		account, err := crypto.AccountFromPrivateKey(expResp.PrivateKey)
		if err != nil {
			log.Fatalf("failed to get account from private key: %v\n", err)
			return nil, err
		}

		accounts = append(accounts, account)
	}

	return accounts, nil
}

func FundAccount(algodClient *algod.Client, recipient string, amount uint64) {
	accounts, err := GetSandboxAccounts()
	if err != nil {
		log.Fatalf("error getting sandbox accounts: %s\n", err)
	}
	fundingAccount := accounts[0]

	sp, err := algodClient.SuggestedParams().Do(context.Background())
	if err != nil {
		log.Fatalf("error getting suggested params: %s\n", err)
	}
	paymenttxn, err := transaction.MakePaymentTxn(fundingAccount.Address.String(), recipient, amount, nil, "", sp)
	if err != nil {
		log.Fatalf("error creating payment txn: %s\n", err)
	}

	// sign the transaction
	_, signed_payment_transaction, err := crypto.SignTransaction(fundingAccount.PrivateKey, paymenttxn)
	if err != nil {
		log.Fatalf("error signing transaction: %s\n", err)
		return
	}

	// submit the transaction
	_, err = algodClient.SendRawTransaction(signed_payment_transaction).Do(context.Background())
	if err != nil {
		log.Fatalf("error sending transaction: %s\n", err)
	}

	// // wait for confirmation
	// confirmedTxn, err := transaction.WaitForConfirmation(algodClient, pendingTransactionID, 4, context.Background())
	// if err != nil {
	// 	log.Fatalf("error confirming transaction: %s\n", err)
	// 	return
	// }
}
