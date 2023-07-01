package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
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

					// parse on chain variables
					onchain_latest_timestamp_bytes := GetValueOfGlobalState(blockchain_app_info.Params.GlobalState, "latest_timestamp")
					if onchain_latest_timestamp_bytes == nil {
						log.Fatalf("Error: latest_timestamp not found in smart contract\n")
						continue
					}
					onchain_latest_timestamp, err := strconv.ParseInt((string(onchain_latest_timestamp_bytes)), 10, 64)
					if err != nil {
						log.Fatalf("Error parsing latest_timestamp: %v\n", err)
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
					// var onchain_partner_balance uint64
					var offchain_my_balance uint64
					// var offchain_partner_balance uint64
					if is_alice {
						onchain_my_balance = onchain_latest_alice_balance
						// onchain_partner_balance = onchain_latest_bob_balance
						offchain_my_balance = latestOffChainState.alice_balance
						// offchain_partner_balance = latestOffChainState.bob_balance
					} else {
						onchain_my_balance = onchain_latest_bob_balance
						// onchain_partner_balance = onchain_latest_alice_balance
						offchain_my_balance = latestOffChainState.bob_balance
						// offchain_partner_balance = latestOffChainState.alice_balance
					}

					if onchain_my_balance > offchain_my_balance {
						// this case is beneficial for me
						fmt.Printf("Latest balances are beneficial for me, I don't want to dispute\n")
						continue
					}

					// if the latest balances are not correct, we need to dispute
					fmt.Printf("Latest balances are not correct, need to dispute\n")
					fmt.Printf("On chain latest timestamp: %v\n", onchain_latest_timestamp)
					fmt.Printf("On chain latest alice balance: %v\n", onchain_latest_alice_balance)
					fmt.Printf("On chain latest bob balance: %v\n", onchain_latest_bob_balance)
					fmt.Printf("Off chain latest timestamp: %v\n", latestOffChainState.timestamp)
					fmt.Printf("Latest alice balance: %v\n", latestOffChainState.alice_balance)
					fmt.Printf("Latest bob balance: %v\n", latestOffChainState.bob_balance)

					// read the latest balances from the blockchain
					fmt.Println("Timeout is ", timeout)
				}
			}

			// sleep for 1 second
			time.Sleep(1 * time.Second)
		}
	}()

}
