# Dockerfile for Payment Channel Participants
# for caching purposes we separate the build stage from the runtime stage

# 1. build stage
FROM golang:1.20.5-bullseye as builder

ENV GODEBUG netdns=cgo

# copy only necessary files to improve caching
COPY    cmd/ascli/ $GOPATH/src/github.com/dancodery/algorand-state-channels/cmd/ascli/
COPY    cmd/asd/ $GOPATH/src/github.com/dancodery/algorand-state-channels/cmd/asd/
COPY    asrpc/ $GOPATH/src/github.com/dancodery/algorand-state-channels/asrpc/
COPY    payment/testing/setup.go $GOPATH/src/github.com/dancodery/algorand-state-channels/payment/testing/
COPY    go.mod go.sum asd.go server.go rpcserver.go config.go $GOPATH/src/github.com/dancodery/algorand-state-channels/

WORKDIR  $GOPATH/src/github.com/dancodery/algorand-state-channels

RUN go build -o  /bin/ascli cmd/ascli/***
RUN go build -o /bin/asd


# 2. runtime stage
FROM debian:bullseye as final

ENV GODEBUG netdns=cgo

# install linux packages
RUN \
    --mount=type=cache,target=/var/cache/apt \
    apt-get update && apt-get install -y \
    netcat-openbsd \
    jq \
    && rm -rf /var/lib/apt/lists/*

COPY    --from=builder /bin/ascli /bin/
COPY    --from=builder /bin/asd /bin/

EXPOSE 28547

CMD /bin/asd
