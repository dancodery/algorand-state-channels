#!/bin/bash

cd /run/user/0
git clone https://github.com/algorand/sandbox.git
cd sandbox/
git checkout eae95b9
./sandbox up -v