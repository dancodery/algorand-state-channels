package main

const (
	DEFAULT_GRPC_PORT = 50051
	DEFAULT_PEER_PORT = 28547
)

type config struct {
	GRPCPort int
	PeerPort int
}

func loadConfig() (*config, error) {
	return &config{
		GRPCPort: DEFAULT_GRPC_PORT,
		PeerPort: DEFAULT_PEER_PORT,
	}, nil
}
