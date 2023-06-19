# Dockerfile for Payment Channel Participants
# for caching purposes we separate the build stage from the runtime stage

# 1. build stage
FROM golang:1.20.5-bullseye as builder

ENV GODEBUG netdns=cgo

# copy only necessary files to improve caching
COPY    cmd/ascli/ $GOPATH/src/github.com/dancodery/algorand-state-channels/cmd/ascli/
COPY    cmd/asd/ $GOPATH/src/github.com/dancodery/algorand-state-channels/cmd/asd/
COPY    asrpc/ $GOPATH/src/github.com/dancodery/algorand-state-channels/asrpc/
COPY    payment/ $GOPATH/src/github.com/dancodery/algorand-state-channels/payment/
COPY    payment/testing/ $GOPATH/src/github.com/dancodery/algorand-state-channels/payment/testing/
COPY    payment/build_contracts/ /build_contracts/
COPY    go.mod go.sum asd.go server.go client.go rpcserver.go config.go $GOPATH/src/github.com/dancodery/algorand-state-channels/

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

RUN mkdir -p /build_contracts

COPY    --from=builder /build_contracts/ /smart_contracts/
COPY    --from=builder /bin/ascli /bin/
COPY    --from=builder /bin/asd /bin/

EXPOSE 28547

CMD /bin/asd
