#!/bin/bash

####### DEFINE FUNCTIONS #######
# Define the help function
help() {
    echo "Usage: $0 --config_file=<file> node1 node2 node3 [-h]"
    echo "Options:"
    echo "  --config_file=<file>: Specify the configuration file"
    echo "  node1 node2 node3: Specify the nodes to use"
    echo "  -h: Show help"
    exit 1
}

# Function to check if all nodes are booted
check_nodes_booted() {
    booted_nodes=0

    while read -r id status; do
        if [[ " ${node_names[@]} " =~ " $id " ]] && [[ $status == "booted" ]]; then
            echo "Testbed node $id is booted."
            ((booted_nodes++))
        fi
    done < <(pos nodes list | awk '{print $1, $3}')
    
    if [[ $booted_nodes -eq ${#node_names[@]} ]]; then
        return 0
    else
        return 1
    fi
}

# Function to check command output for specific keywords
check_command_output() {
    local command="$1"
    local keywords="$2"

    output=$(eval "$command")

    echo "$output" 
    # Check if the keyword is present in the output
    if echo "$output" | grep -q "$keyword"; then
        return 0  # Keyword found, command is ready
    else
        return 1  # Keyword not found, command is not ready
    fi
}

# Function to convert FQDN to IP address
fqdn_to_ip() {
    local fqdn=$1
    local ip=$(dig +short $fqdn)
    echo "$ip"
}

####### END DEFINE FUNCTIONS #######

# print out executed commands
# set -x

# stop program in case of error
set -e

# 1. Read node arguments into an array

# Parse command-line arguments
CONFIG_FILE=""
node_names=()
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
            node_names+=("$arg")
			;;
	esac
done

# Loading configuration file
if [ -z "$CONFIG_FILE" ]; then
    help
	exit 1
fi
source "$CONFIG_FILE"

echo "Configuration values:"
echo "======================================================"
echo "funding_amount=$funding_amount"
echo "penalty_reserve=$penalty_reserve"
echo "dispute_window=$dispute_window"
echo "alice_to_bob_payment_rounds=$alice_to_bob_payment_rounds"
echo "bob_to_alice_payment_rounds=$bob_to_alice_payment_rounds"
echo "payment_amount=$payment_amount"
echo

# 2. Free hosts
for ((i=0; i<${#node_names[@]}; i++)); do
    echo "Freeing node ${node_names[i]}..."
    pos allocations free -k ${node_names[i]}
done
echo

# 3. Allocate nodes
echo "Allocating nodes ${node_names[@]}..."
allocate_output=$(pos allocations allocate "${node_names[@]}")

# 4. Save variables
allocation_id=$(echo "$allocate_output" | awk -F ': ' '/Allocation ID:/ {gsub(/[()]/,"",$2); print $2}')
result_directory=$(echo "$allocate_output" | awk '/Results in/ {print $NF}')
echo "Allocation ID: $allocation_id"
echo "Result Directory: $result_directory"
echo

# 5. Load images for nodes individually
for ((i=0; i<${#node_names[@]}; i++)); do
    # load image
    echo "Loading image for node ${node_names[i]}..."
    pos nodes image ${node_names[i]} debian-bullseye

    # 6. reset node and reboot
    echo "Reset node ${node_names[i]}..."
    pos nodes reset --non-blocking ${node_names[i]} 

    echo
done


# 7. Wait for nodes to be booted
echo "Waiting for nodes to boot..."
while ! check_nodes_booted; do
    sleep 5
done
echo

# 8. Save node IPs
sandbox_ip=$(fqdn_to_ip "${node_names[0]}.blockchain.net.in.tum.de")
alice_ip=$(fqdn_to_ip "${node_names[1]}.blockchain.net.in.tum.de")
bob_ip=$(fqdn_to_ip "${node_names[2]}.blockchain.net.in.tum.de")

# Print the IP addresses
echo "Sandbox IP: $sandbox_ip"
echo "Alice IP: $alice_ip"
echo "Bob IP: $bob_ip"
echo

for ((i=0; i<${#node_names[@]}; i++)); do
    # 9. Copy files to nodes
    echo "Copying files to node ${node_names[i]}..."
    pos nodes copy --recursive --dest /root ${node_names[i]} /home/gockel/algorand-state-channels

    echo
done


# 10. Setup Docker for alice and bob
alice_node=${node_names[1]}
bob_node=${node_names[2]}

echo "Installing Docker on node ${alice_node}..."
pos commands launch --infile testbed/install_docker.sh --name docker-setup ${alice_node}

echo "Installing Docker on node ${bob_node}..."
pos commands launch --infile testbed/install_docker.sh --name docker-setup ${bob_node}


# 11. Setup algorand sandbox
sandbox_node=${node_names[0]}

echo "Extending file system on node ${sandbox_node}..."
pos commands launch --infile testbed/extend_filesystem.sh --name extend-filesystem ${sandbox_node}

echo "Installing docker on node ${sandbox_node}..."
pos commands launch --infile testbed/install_docker.sh --name docker-setup ${sandbox_node}

echo "Running algorand sandbox on node ${sandbox_node}..."
pos commands launch --infile testbed/run_sandbox.sh --name run-sandbox ${sandbox_node}

echo


# 12. Start alice and bob nodes
echo "Starting node ${alice_node}..."
pos commands launch --infile testbed/start_node.sh --name start-node ${alice_node}
pos commands launch --name run-container ${alice_node} -- docker run -d --name asc-my-node -p 28547:28547 \
                                            -e ALGOD_ADDRESS="http://${sandbox_ip}:4001" \
                                            -e KMD_ADDRESS="http://${sandbox_ip}:4002" \
                                            -e INDEXER_ADDRESS="http://${sandbox_ip}:8980" \
                                                asc-my-node

echo "Starting node ${bob_node}..."
pos commands launch --infile testbed/start_node.sh --name start-node ${bob_node}
pos commands launch --name run-container ${bob_node} -- docker run -d --name asc-my-node -p 28547:28547 \
                                            -e ALGOD_ADDRESS="http://${sandbox_ip}:4001" \
                                            -e KMD_ADDRESS="http://${sandbox_ip}:4002" \
                                            -e INDEXER_ADDRESS="http://${sandbox_ip}:8980" \
                                                asc-my-node

echo 

# 13. Start measurements
echo "Starting measurements..."
source testbed/run_measurements.sh
