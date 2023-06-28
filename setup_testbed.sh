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
pos allocations allocate ${args[@]}

# 4. Configure nodes individually
for ((i=0; i<${#args[@]}; i++)); do
    echo "Configuring node ${args[i]}..."
    pos nodes image ${args[i]} debian-bullseye

    echo "Reset node ${args[i]}..."
    pos nodes reset ${args[i]}

    echo "Launching commands on node ${args[i]}"
    pos commands launch ${args[i]} -- echo "$(hostname)"

    echo 
done

# 5. Print result folder
allocation_id=$(pos allocations list | grep "gockel" | awk '{ print $1 }')
echo "Allocation ID: $allocation_id"


# results dir: /srv/testbed/results/gockel/default