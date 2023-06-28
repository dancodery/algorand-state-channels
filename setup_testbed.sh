#!/bin/bash


# 1. Read node arguments into an array
args=("$@")

# 2. Allocate nodes
for ((i=0; i<${#args[@]}; i++)); do
    echo 
    echo "Freeing node ${args[i]}..."
    pos allocations free ${args[i]}
    
    echo "Allocating node ${args[i]}..."
    pos allocations allocate ${args[i]}

    echo "Configuring node ${args[i]}"
    pos nodes image ${args[i]} debian-bullseye
done



# 3. Configure nodes

# debian-bullseye