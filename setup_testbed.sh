#!/bin/bash


# 1. Read node arguments into an array
args=("$@")

# Echo the arguments
for ((i=0; i<${#args[@]}; i++)); do
    echo "$((i+1)). argument ${args[i]}"
done


# 2. Allocate nodes

# 3. Configure nodes

# debian-bullseye