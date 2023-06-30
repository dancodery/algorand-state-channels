#!/bin/bash

echo "Hello from sandbox script"

cd algorand-state-channels/
touch .env
./sandbox up -v