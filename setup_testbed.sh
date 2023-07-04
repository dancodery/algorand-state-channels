#!/bin/bash

# print out executed commands
# set -x

# stop program in case of error
set -e

# 1. Read node arguments into an array
args=("$@")

check_nodes_booted() {
    booted_nodes=0

    while read -r id status; do
        if [[ " ${args[@]} " =~ " $id " ]] && [[ $status == "booted" ]]; then
            echo "Testbed node $id is booted."
            ((booted_nodes++))
        fi
    done < <(pos nodes list | awk '{print $1, $3}')
    
    if [[ $booted_nodes -eq ${#args[@]} ]]; then
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

# 2. Free hosts
for ((i=0; i<${#args[@]}; i++)); do
    echo "Freeing node ${args[i]}..."
    pos allocations free -k ${args[i]}
done
echo

# 3. Allocate nodes
echo "Allocating nodes ${args[@]}..."
allocate_output=$(pos allocations allocate "${args[@]}")

# 4. Save variables
allocation_id=$(echo "$allocate_output" | awk -F ': ' '/Allocation ID:/ {gsub(/[()]/,"",$2); print $2}')
result_directory=$(echo "$allocate_output" | awk '/Results in/ {print $NF}')
echo "Allocation ID: $allocation_id"
echo "Result Directory: $result_directory"
echo

# 5. Load images for nodes individually
for ((i=0; i<${#args[@]}; i++)); do
    # load image
    echo "Loading image for node ${args[i]}..."
    pos nodes image ${args[i]} debian-bullseye

    # 6. reset node and reboot
    echo "Reset node ${args[i]}..."
    pos nodes reset --non-blocking ${args[i]} 

    echo
done


# 7. Wait for nodes to be booted
echo "Waiting for nodes to boot..."
while ! check_nodes_booted; do
    sleep 5
done
echo


for ((i=0; i<${#args[@]}; i++)); do
    # 8. Copy files to nodes
    echo "Copying files to node ${args[i]}..."
    pos nodes copy --recursive --dest /root ${args[i]} /home/gockel/algorand-state-channels

    echo
done


# 9. Setup Docker for alice and bob
alice_node=${args[1]}
bob_node=${args[2]}

echo "Installing Docker on node ${alice_node}..."
pos commands launch --infile testbed/install_docker.sh --queued --name docker-setup ${alice_node}

echo "Installing Docker on node ${bob_node}..."
pos commands launch --infile testbed/install_docker.sh --queued --name docker-setup ${bob_node}


# 10. Setup algorand sandbox
sandbox_node=${args[0]}

echo "Extending file system on node ${sandbox_node}..."
pos commands launch --infile testbed/extend_filesystem.sh --name extend-filesystem ${sandbox_node}

echo "Installing Docker on node ${sandbox_node}..."
pos commands launch --infile testbed/install_docker.sh --name docker-setup ${sandbox_node}

echo "Running sandbox on node ${sandbox_node}..."
pos commands launch --infile testbed/run_sandbox.sh --name run-sandbox ${sandbox_node}

echo


# 11. Start nodes
echo "Starting node ${alice_node}..."
pos commands launch --infile testbed/start_node.sh --queued --name start-node ${alice_node}

echo "Starting node ${bob_node}..."
pos commands launch --infile testbed/start_node.sh --queued --name start-node ${bob_node}

echo 
