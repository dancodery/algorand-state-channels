#!/bin/bash

cd algorand-state-channels/
docker build -t asc-my-node .
docker run -d --name asc-my-node -p 28547:28547 \
    -e ALGOD_ADDRESS="dogecoin.blockchain.net.in.tum.de:4001" \
    -e KMD_ADDRESS="dogecoin.blockchain.net.in.tum.de:4002" \
    -e INDEXER_ADDRESS="dogecoin.blockchain.net.in.tum.de:8980" \
        asc-my-node
