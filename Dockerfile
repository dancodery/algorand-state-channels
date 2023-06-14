# Dockerfile for Payment Channel Participants
FROM golang:1.20.5-bullseye

ENV GODEBUG netdns=cgo

# install linux packages
RUN apt-get update && apt-get install -y \
    netcat-openbsd \
    && rm -rf /var/lib/apt/lists/*

COPY .  $GOPATH/src/github.com/dancodery/algorand-state-channels
WORKDIR  $GOPATH/src/github.com/dancodery/algorand-state-channels

RUN go build -o  $GOPATH/bin/ascli cmd/ascli/***
RUN go build -o  $GOPATH/bin/asd config.go asd.go server.go

CMD $GOPATH/bin/asd



####### DEPRECATED ########
# FROM python:3.11
# required; otherwise, Python output will not be printed to the console
# ENV PYTHONUNBUFFERED=1
# WORKDIR /algorand-state-channel

# Copy only the requirements file first and install dependencies
# This step will be cached if the requirements.txt file doesn't change
# --no-cache-dir is used to avoid caching issues
# COPY requirements.txt .
# RUN pip install -r requirements.txt

# Copy the remaining code files
# COPY state_channel_node.py .
# COPY payment/ payment/                

# Set entrypoint
# TODO: use config file config.yml
# ENTRYPOINT ["python", "state_channel_node.py", "runserver"] 