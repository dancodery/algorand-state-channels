package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/dancodery/algorand-state-channels/payment"
)

func (s *server) UpdateWatchtowerState() {
	go func() {
		for {
			for address, payment_channel_onchain_state := range s.payment_channels_onchain_states {
				// read smart contract from the blockchain for given app_id
				blockchain_app_info, err := s.algod_client.GetApplicationByID(payment_channel_onchain_state.app_id).Do(context.Background())
				if err != nil {
					log.Fatalf("Error reading smart contract from blockchain: %v\n", err)
					return
				}
				// fmt.Printf("Smart contract info: %+v\n", blockchain_app_info)
				// check if the channel is in the closing phase
				timeout_bytes := GetValueOfGlobalState(blockchain_app_info.Params.GlobalState, "timeout")
				if timeout_bytes == nil {
					continue
				}
				timeout := parseInt(string(timeout_bytes))

				// if closing was initiated
				if timeout > 0 {
					// find out if I am alice or bob
					var is_alice bool
					if s.algo_account.Address.String() == payment_channel_onchain_state.alice_address {
						is_alice = true
					} else {
						is_alice = false
					}

					// get latest off chain state
					payment_log, ok := s.payment_channels_offchain_states_log[address]
					if !ok {
						fmt.Printf("Error: payment channel with partner node %v does not exist\n", address)
						continue
					}
					latestOffChainState, err := getLatestOffChainState(payment_log)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						continue
					}

					onchain_latest_alice_balance_bytes := GetValueOfGlobalState(blockchain_app_info.Params.GlobalState, "latest_alice_balance")
					if onchain_latest_alice_balance_bytes == nil {
						log.Fatalf("Error: latest_alice_balance not found in smart contract\n")
						continue
					}
					onchain_latest_alice_balance, err := strconv.ParseUint((string(onchain_latest_alice_balance_bytes)), 10, 64)
					if err != nil {
						log.Fatalf("Error parsing latest_alice_balance: %v\n", err)
						continue
					}

					onchain_latest_bob_balance_bytes := GetValueOfGlobalState(blockchain_app_info.Params.GlobalState, "latest_bob_balance")
					if onchain_latest_bob_balance_bytes == nil {
						log.Fatalf("Error: latest_bob_balance not found in smart contract\n")
						continue
					}
					onchain_latest_bob_balance, err := strconv.ParseUint((string(onchain_latest_bob_balance_bytes)), 10, 64)
					if err != nil {
						log.Fatalf("Error parsing latest_bob_balance: %v\n", err)
						continue
					}

					// check if the onchainstate is beneficial for me
					var onchain_my_balance uint64
					var offchain_my_balance uint64
					if is_alice {
						onchain_my_balance = onchain_latest_alice_balance
						offchain_my_balance = latestOffChainState.alice_balance
					} else {
						onchain_my_balance = onchain_latest_bob_balance
						offchain_my_balance = latestOffChainState.bob_balance
					}

					if onchain_my_balance >= offchain_my_balance {
						// this case is beneficial for me
						fmt.Printf("Latest balances are beneficial for me, I don't want to dispute\n\n")
						continue
					}

					// if the latest balances are not correct, we need to dispute
					fmt.Printf("Latest balances are not beneficial for me, I want to dispute\n\n")

					payment.RaiseDispute(
						s.algod_client,
						s.algo_account,
						4161,
						payment_channel_onchain_state.app_id,
						latestOffChainState.alice_balance,
						latestOffChainState.bob_balance,
						uint64(latestOffChainState.timestamp),
						latestOffChainState.alice_signature,
						latestOffChainState.bob_signature)

					fmt.Printf("On chain state alice balance: %v\n", onchain_latest_alice_balance)
					fmt.Printf("On chain state bob balance: %v\n", onchain_latest_bob_balance)
					fmt.Printf("Disputed real alice balance: %v\n", latestOffChainState.alice_balance)
					fmt.Printf("Disputed real bob balance: %v\n\n", latestOffChainState.bob_balance)

					// delete the payment channel from the list of payment channels
					delete(s.payment_channels_onchain_states, address)
				}
			}

			// sleep for 1 second
			time.Sleep(1 * time.Second)
		}
	}()

}
