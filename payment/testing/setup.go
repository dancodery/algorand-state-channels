package testing

import (
	"log"

	"github.com/algorand/go-algorand-sdk/client/algod"
	"github.com/algorand/go-algorand-sdk/v2/client/kmd"
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
	return &algodClient
}

func GetKmdClient() *algod.Client {
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
