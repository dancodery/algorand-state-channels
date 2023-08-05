# Dockerfile for Payment Channel Participants
# for caching purposes we separate the build stage from the runtime stage

# 1. build stage
FROM golang:1.20.5-bullseye as builder

ENV GODEBUG netdns=cgo

WORKDIR  $GOPATH/src/github.com/dancodery/algorand-state-channels

# copy mod.go and sum.go to improve caching
COPY    go.mod go.sum ./

# download dependencies
RUN go mod download

# copy only necessary files to improve caching
COPY    cmd/ascli/ ./cmd/ascli/
COPY    cmd/ascli/ $GOPATH/src/github.com/dancodery/algorand-state-channels/cmd/ascli/
COPY    asrpc/ $GOPATH/src/github.com/dancodery/algorand-state-channels/asrpc/
COPY    payment/ $GOPATH/src/github.com/dancodery/algorand-state-channels/payment/
COPY    payment/testing/ $GOPATH/src/github.com/dancodery/algorand-state-channels/payment/testing/
COPY    payment/build_contracts/ /smart_contracts/
COPY    asd.go server.go client.go rpcserver.go config.go watchtower.go $GOPATH/src/github.com/dancodery/algorand-state-channels/

# build binaries
RUN go build -o /bin/ascli cmd/ascli/***
RUN go build -o /bin/asd


# 2. run stage
FROM debian:bullseye as final

# needed for docker network
ENV GODEBUG netdns=cgo

# install linux packages
RUN \
    --mount=type=cache,target=/var/cache/apt \
    apt-get update && apt-get install -y \
    netcat-openbsd \
    jq \
    && rm -rf /var/lib/apt/lists/*

# copy binaries to final image
COPY    --from=builder /smart_contracts/ /smart_contracts/
COPY    --from=builder /bin/ascli /bin/
COPY    --from=builder /bin/asd /bin/

# expose p2p port
EXPOSE 28547

# run the server
CMD /bin/asd
