#!/bin/bash


# 1. Read node arguments into an array
args=("$@")

# 2. Allocate nodes
echo "Allocating nodes ${args[@]}..."
pos allocations allocate ${args[@]}

# 3. Configure nodes
for ((i=0; i<${#args[@]}; i++)); do
    echo "Configuring node ${args[i]}..."
    pos nodes image ${args[i]} debian-bullseye

    # echo "Reset node ${args[i]}..."
    # pos nodes reset ${args[i]}

    # echo "Launching commands on node ${args[i]}"
    # pos commands launch ${args[i]} "echo \$(hostname)"
done



# 3. Configure nodes

# debian-bullseye