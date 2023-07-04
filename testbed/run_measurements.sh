#!/bin/bash

echo "Hello from measurements script!"
docker exec -it asc-my-node ascli getinfo
