#!/bin/bash

# Helper functions
# run-in-node: Run a command inside a docker container, using the bash shell
function run-in-node () {
	pos commands launch -v "$1" -- docker exec asc-my-node /bin/bash -c "${@:2}"
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

### Start the measurements ###
echo "Starting the measurements..."
echo "======================================================"
echo 

# Wait for nodes to be ready
echo "Waiting for nodes to be ready..."
wait-for-node ${alice_node} docker exec -it asc-my-node ascli ascli getinfo
wait-for-node ${bob_node} docker exec -it asc-my-node ascli ascli getinfo

echo "Hello from measurements script!"
# docker exec -it asc-my-node ascli getinfo

# Resetting Alice and Bob's nodes
echo "Resetting Alice and Bob's nodes..."
run-in-node ${alice_node} "ascli reset"
run-in-node ${bob_node} "ascli reset"
echo