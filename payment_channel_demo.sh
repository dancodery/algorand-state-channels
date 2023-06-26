#!/bin/bash

# file based on https://github.com/lnbook/lnbook

# Define the help function
help() {
    echo "Usage: $0 --config_file=<file> [-h]"
    echo "Options:"
    echo "  --config_file=<file>: Specify the configuration file"
    echo "  -h: Show help"
    exit 1
}


# Helper functions
# run-in-node: Run a command inside a docker container, using the bash shell
function run-in-node () {
	docker exec "$1" /bin/bash -c "${@:2}"
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

# Parse command-line arguments
for arg in "$@"; do
	case $arg in
		--config_file=*)
			CONFIG_FILE="${arg#*=}"
			;;
		-h|--help)
			help
			exit 0
			;;
		*)
			echo "Invalid argument: $arg"
			echo
			help
			exit 1
			;;
	esac
done

# Loading configuration file
if [ -z "$CONFIG_FILE" ]; then
    help
	exit 1
fi
source "$CONFIG_FILE"

# echo "Configuration values:"
# echo "runs=$runs"
# echo "port=$port"
# echo "outfile=$outfile"
# echo

### Start the demo ###
echo "Starting the demo..."
echo "======================================================"
echo 

# Alice: get algo address
echo "Getting algo address from Alice..."
alice_address=$(run-in-node asc-alice "ascli getinfo | jq -r .algo_address") # save Alice's address as raw string
# Print Alice and Bob's addresses
echo "Alice's address: ${alice_address}"

# Bob: get algo address
echo "Getting algo address from Bob..."
bob_address=$(run-in-node asc-bob "ascli getinfo | jq -r .algo_address") # save Bob's address as raw string
echo "Bob's address: ${bob_address}"

# Alice: open a channel with Bob
echo 
echo "Alice opening a channel with Bob..."
run-in-node asc-alice "ascli openchannel --partner_ip=asc-bob --partner_address=${bob_address} --funding_amount=2_000_000_000 --penalty_reserve=100_000 --dispute_window=1000"

# Alice pays to Bob 100 microAlgos
echo
echo "Alice paying Bob 100 microAlgos..."
run-in-node asc-alice "ascli pay --partner_address=${bob_address} --amount=100"

# Alice pays again to Bob 100 microAlgos
echo
echo "Alice paying Bob 100 microAlgos..."
run-in-node asc-alice "ascli pay --partner_address=${bob_address} --amount=100"

# Close the channel
echo
echo "Alice closing the channel..."
run-in-node asc-alice "ascli closechannel --partner_address=${bob_address}"