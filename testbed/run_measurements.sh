#!/bin/bash

# Helper functions
# run-in-node: Run a command inside a docker container, using the bash shell
function run-in-node () {
	# docker exec -it asc-my-node ascli getinfo
	# docker exec "$1" /bin/bash -c "${@:2}"
	pos commands launch -v $1 -- docker exec asc-my-node /bin/bash -c "${@:2}"
	echo "pos commands launch -v $1 -- docker exec asc-my-node /bin/bash -c \"${@:2}\""
}

# wait-for-cmd: Run a command repeatedly until it completes/exits successfuly
function wait-for-cmd () {
		until "${@}" > /dev/null 2>&1
		do
			echo -n "."
			sleep 1
		done
		echo
}

function wait-for-node () {
	wait-for-cmd run-in-node $1 "${@:2}"
}

### Start the measurements ###
echo "Starting the measurements..."
echo "======================================================"
echo 

# Wait for nodes to be ready
echo "Waiting for nodes to be ready..."
wait-for-node ${alice_node} "ascli getinfo"
wait-for-node ${bob_node} "ascli getinfo"

# Resetting Alice and Bob's nodes
echo "Resetting Alice and Bob's nodes..."
run-in-node ${alice_node} "ascli reset"
run-in-node ${bob_node} "ascli reset"
echo


# Print Alice and Bob's addresses

# Alice: get algo address
echo "Getting algo address from Alice..."
alice_address=$(run-in-node ${alice_node} "ascli getinfo | jq -r .algo_address") # save Alice's address as raw string
alice_starting_balance=$(run-in-node ${alice_node} "ascli getinfo | jq -r .algo_balance") # save Alice's balance as raw string
echo "Alice's address: ${alice_address}"
echo "Alice's starting balance: ${alice_starting_balance}"
echo 

# Bob: get algo address
echo "Getting algo address from Bob..."
bob_address=$(run-in-node ${bob_node} "ascli getinfo | jq -r .algo_address") # save Bob's address as raw string
bob_starting_balance=$(run-in-node ${bob_node} "ascli getinfo | jq -r .algo_balance") # save Bob's balance as raw string
echo "Bob's address: ${bob_address}"
echo "Bob's starting balance: ${bob_starting_balance}"

# Alice: open a channel with Bob
echo 
echo "Alice opening a channel with Bob..."
run-in-node ${alice_node} "ascli openchannel --partner_ip=${bob_node} --partner_address=${bob_address} --funding_amount=${funding_amount} --penalty_reserve=${penalty_reserve} --dispute_window=${dispute_window}"

# Make payments from Alice to Bob
for ((i=1; i<=${alice_to_bob_payment_rounds}; i++)); do
	echo
	echo "Alice paying Bob ${payment_amount} microAlgos (round ${i})..."
	run-in-node ${alice_node} "ascli pay --partner_address=${bob_address} --amount=${payment_amount}"
done

# Make payments from Bob to Alice
for ((i=1; i<=${bob_to_alice_payment_rounds}; i++)); do
	echo
	echo "Bob paying Alice ${payment_amount} microAlgos (round ${i})..."
	run-in-node ${bob_node} "ascli pay --partner_address=${alice_address} --amount=${payment_amount}"
done

# # Bob tries to cheat by closing the channel with an old state
# echo
# echo "Bob trying to cheat by closing the channel with an old state..."
# run-in-node ${bob_node} "ascli trytocheat --partner_address=${alice_address}"

# Bob closes the channel cooperatively
echo 
echo "Bob closing the channel cooperatively..."
run-in-node ${bob_node} "ascli cooperativeclosechannel --partner_address=${alice_address}"

# # Initiate closing the channel
# echo
# echo "Alice initiating channel closing..."
# run-in-node ${alice_node} "ascli initiateclosechannel --partner_address=${bob_address}"

# # sleep for dispute_window * block_time
# echo
# echo "Waiting for dispute window to expire: ${dispute_window} * 4 seconds..."
# sleep $(echo "${dispute_window} * 4" | bc)

# # Finalize closing the channel
# echo
# echo "Bob finalizing channel closing..."
# run-in-node ${alice_node} "ascli finalizeclosechannel --partner_address=${bob_address}"

# Get Alice and Bob's final balances
alice_final_balance=$(run-in-node ${alice_node} "ascli getinfo | jq -r .algo_balance") # save Alice's balance as raw string
bob_final_balance=$(run-in-node ${bob_node} "ascli getinfo | jq -r .algo_balance") # save Bob's balance as raw string
echo 
echo "======================================================"
echo "Alice's final balance: ${alice_final_balance}"
echo "Bob's final balance: ${bob_final_balance}"

total_transaction_fees=$(echo "${alice_starting_balance} - ${alice_final_balance} + ${bob_starting_balance} - ${bob_final_balance}" | bc)
echo "Total transaction fees: ${total_transaction_fees}"