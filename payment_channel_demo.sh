#!/bin/bash
# file based on https://github.com/lnbook/lnbook

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
while [[ $# -gt 0 ]]; do
	key="$1"
	case $key in
		--config_file)
			config_file="$2"
			shift
			;;
		*)
			echo "Invalid argument: $key"
			exit 1
			;;
	esac

	shift
done

# Loading configuration file
if [[ -z "$config_file" ]]; then
	echo "No configuration file specified. Using default configuration."
	echo "Usage: ./payment_channel_demo.sh --config_file=<config_file>"
	exit 1
fi
source "$config_file"

echo "Configuration values:"
echo "runs=$runs"
echo "port=$port"
echo "outfile=$outfile"
echo

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
run-in-node asc-alice "ascli openchannel --partner_ip=asc-bob --partner_address=${bob_address} --funding_amount=10_000_000 --penalty_reserve=100_000 --dispute_window=1000"


    # cli.py openchannel --partner="bob_address" --funding=1000000 --penalty_reserve=100000 --dispute_window=1000
