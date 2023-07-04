#!/bin/bash

cd algorand-state-channels/
docker build -t asc-my-node .
docker run -d --name asc-my-node -p 28547:28547 \
    -e ALGOD_ADDRESS="http://${sandbox_ip}:4001" \
    -e KMD_ADDRESS="http://${sandbox_ip}:4002" \
    -e INDEXER_ADDRESS="http://${sandbox_ip}:8980" \
        asc-my-node
