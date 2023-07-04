#!/bin/bash

cd algorand-state-channels/
docker build -t asc-my-node .
docker run -d --name asc-my-node -p 28547:28547 \
    -e ALGOD_ADDRESS="172.16.158.1:4001" \
    -e KMD_ADDRESS="172.16.158.1:4002" \
    -e INDEXER_ADDRESS="172.16.158.1:8980" \
        asc-my-node
