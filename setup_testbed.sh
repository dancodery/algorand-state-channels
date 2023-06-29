#!/bin/bash

# print out executed commands
# set -x

# stop program in case of error
set -e


# 1. Read node arguments into an array
args=("$@")

# 2. Free hosts
for ((i=0; i<${#args[@]}; i++)); do
    echo "Freeing node ${args[i]}..."
    pos allocations free -k ${args[i]}
done

# 3. Allocate nodes
echo "Allocating nodes ${args[@]}..."
allocate_output=$(pos allocations allocate "${args[@]}")
echo
echo "$allocate_output"
echo
sleep 1

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

    # reset node and reboot
    echo "Reset node ${args[i]}..."
    pos nodes reset --non-blocking ${args[i]} 

    echo
done

# 6. Wait for nodes to be ready, so that they can reboot in parallel
echo "Waiting for nodes to be ready..."
sleep 100 # 90 works, 90 not
echo

for ((i=0; i<${#args[@]}; i++)); do
    # 7. Copy files to nodes
    echo "Copying files to node ${args[i]}..."
    pos nodes copy --recursive --dest /root ${args[i]} /home/gockel/algorand-state-channels

    # 8. Run commands on nodes
    # echo "Running commands on node ${args[i]}..."
    # pos commands launch ${args[i]} -- echo "$(hostname)"
    # pos commands launch ${args[i]} -- /bin/bash -c "cd /root/algorand-state-channels && ./setup.sh"
done

# 9. Setup algorand sandbox
sandbox_node=${args[0]}
pos commands launch sandbox_node -- apt update
pos commands launch sandbox_node -- apt upgrade
pos commands launch sandbox_node -- apt-get install \
                                                ca-certificates \
                                                curl \
                                                gnupg


# 10. Setup for alice and bob


echo 
