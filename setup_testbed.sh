#!/bin/bash


# 1. Read node arguments into an array
args=("$@")

# 2. Allocate nodes
echo "Allocating nodes ${args[@]}..."
pos allocations allocate ${args[@]}

# 3. Configure nodes individually
for ((i=0; i<${#args[@]}; i++)); do
    echo "Configuring node ${args[i]}..."
    pos nodes image ${args[i]} debian-bullseye

    echo "Reset node ${args[i]}..."
    pos nodes reset ${args[i]} --non-blocking

    echo "Launching commands on node ${args[i]}"
    pos commands launch ${args[i]} -- echo "$(hostname)"
done

# 4. Wait for nodes to be ready
echo "Waiting for nodes to be ready..."
sleep 10
