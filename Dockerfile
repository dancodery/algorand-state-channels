# Dockerfile for Payment Channel Participants
# for caching purposes we separate the build stage from the runtime stage

# 1. build stage
FROM golang:1.20.5-bullseye as builder

ENV GODEBUG netdns=cgo

# install linux packages
RUN apt-get update && apt-get install -y \
    netcat-openbsd \
    && rm -rf /var/lib/apt/lists/*

COPY .  $GOPATH/src/github.com/dancodery/algorand-state-channels
WORKDIR  $GOPATH/src/github.com/dancodery/algorand-state-channels

RUN go build -o  $GOPATH/bin/ascli cmd/ascli/***
RUN go build -o $GOPATH/bin/asd
# asd.go config.go server.go rpcserver.go asrpc/*.g


# 2. runtime stage
FROM debian:bullseye as final

COPY --from=builder /go/bin/ascli /bin/
COPY --from=builder /go/bin/asd /bin/

EXPOSE 28547

CMD /bin/asd
