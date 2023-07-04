#!/bin/bash

cd algorand-state-channels/
docker build -t asc-my-node .
docker run -d --name asc-my-node -p 28547:28547 asc-my-node
