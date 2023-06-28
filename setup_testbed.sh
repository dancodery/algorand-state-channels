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
allocation_id=$(echo "$allocate_output" | awk '/Allocation ID:/ {print $NF}')
result_directory=$(echo "$allocate_output" | awk '/Results in/ {print $NF}')
echo "Allocation ID: $allocation_id"
echo "Result Directory: $result_directory"
echo

# 4. Configure nodes individually
for ((i=0; i<${#args[@]}; i++)); do
    # load image
    echo "Configuring node ${args[i]}..."
    pos nodes image ${args[i]} debian-bullseye

    # reset node and reboot
    echo "Reset node ${args[i]}..."
    pos nodes reset ${args[i]}

    # copy files to node
    echo "Copying files to node ${args[i]}..."    
    pos nodes copy --recursive --dest /root ${args[i]} /home/gockel/algorand-state-channels

    # launch commands
    echo "Launching commands on node ${args[i]}"
    pos commands launch ${args[i]} -- echo "$(hostname)"

    echo 
done



# results dir: /srv/testbed/results/gockel/default