#!/bin/bash

# Helper functions
# run-in-node: Run a command inside a docker container, using the bash shell
function run-in-node () {
	# docker exec -it asc-my-node ascli getinfo
	# docker exec "$1" /bin/bash -c "${@:2}"
	pos commands launch -v $1 -- docker exec asc-my-node /bin/bash -c "${@:2}"
	# echo "pos commands launch -v $1 -- docker exec asc-my-node /bin/bash -c \"${@:2}\""
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

function calculate_runtime_difference() {
	local response="$1"
	local runtime_recording=$(echo "$response" | awk -F 'runtime_recording:{' '{print $2}' | sed 's/}[^}]*$//')
	local timestamp_start=$(echo "$runtime_recording" | awk -F '[: }]+' '/timestamp_start/{printf "%.9f", $3 + $5/1000000000}')
	local timestamp_end=$(echo "$runtime_recording" | awk -F '[: }]+' '/timestamp_end/{printf "%.9f", $8 + $10/1000000000}')
	# local difference=$(awk -v start="$timestamp_start" -v end="$timestamp_end" 'BEGIN { diff = end - start; print diff }')
	local difference=$(echo "$timestamp_end - $timestamp_start" | bc)

	echo $difference
}

### Start the measurements ###
echo "Starting the measurements..."
echo "======================================================"
echo 

# Wait for nodes to be ready
echo "Waiting for nodes to be ready..."
wait-for-node ${alice_node} "ascli getinfo"
wait-for-node ${bob_node} "ascli getinfo"

payments_1000_record="{"

how_many_payments_final=1000
echo "Amount of payments: ${how_many_payments_final}"
echo "========================="
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
channel_open_response=$(run-in-node ${alice_node} "ascli openchannel --partner_ip=${bob_node} --partner_address=${bob_address} --funding_amount=${funding_amount} --penalty_reserve=${penalty_reserve} --dispute_window=${dispute_window}")
echo $channel_open_response

# Make payments from Alice to Bob
for ((i=1; i<=${how_many_payments_final}; i++)); do
	echo
	echo "Alice paying Bob ${payment_amount} microAlgos (round ${i})..."
	pay_response=$(run-in-node ${alice_node} "ascli pay --partner_address=${bob_address} --amount=${payment_amount}")
	echo $pay_response
	pay_difference=$(calculate_runtime_difference "$pay_response")
	if (( $(echo "$pay_difference < 1" | bc -l) )); then
		pay_difference="0$pay_difference"
	fi
	echo "Pay difference: $pay_difference"
	payments_1000_record+="  \"${i}\": {\"execution_time\": ${pay_difference}},"
done


# Bob closes the channel cooperatively
echo 
echo "Bob closing the channel cooperatively..."
cooperative_close_response=$(run-in-node ${bob_node} "ascli cooperativeclosechannel --partner_address=${alice_address}")
echo $cooperative_close_response

# Get Alice and Bob's final balances
alice_final_balance=$(run-in-node ${alice_node} "ascli getinfo | jq -r .algo_balance") # save Alice's balance as raw string
bob_final_balance=$(run-in-node ${bob_node} "ascli getinfo | jq -r .algo_balance") # save Bob's balance as raw string
echo 
echo "============="
echo "Alice's final balance: ${alice_final_balance} microAlgos"
echo "Bob's final balance: ${bob_final_balance} microAlgos"

echo
total_transaction_fees=$(echo "${alice_starting_balance} - ${alice_final_balance} + ${bob_starting_balance} - ${bob_final_balance}" | bc)
echo "Total transaction fees: ${total_transaction_fees} microAlgos"
echo

# Remove trailing comma and close payments object
payments_1000_record=${payments_1000_record%?}	
payments_1000_record+="}"

## Save results
# Create parent directories if they do not exist
mkdir -p testbed/results

# Extract results_filename from CONFIG_FILE variable
results_filename=$(basename "$CONFIG_FILE")
results_filename="${results_filename%.*}"

json_content="{
\"funding_amount\": $(echo "$funding_amount" | tr -d '_'),
\"penalty_reserve\": ${penalty_reserve},
\"dispute_window\": ${dispute_window},

\"dispute_probability\": ${dispute_probability},

\"payments_1000\": ${payments_1000_record}
}"

# \"total_transaction_fees\": ${total_transaction_fees}

# Save JSON content to file in testbed/results/${outfile}
echo "$json_content" > "testbed/results/${results_filename}.json"


