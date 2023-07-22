#!/bin/bash

git clone https://github.com/algorand/sandbox.git
cd sandbox/
git checkout eae95b9
file_path="images/algod/DevModeNetwork.json"
sed -i 's/"DevMode": true/"DevMode": false/' "$file_path"
./sandbox up dev -v