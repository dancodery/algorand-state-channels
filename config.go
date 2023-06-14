package main

const (
	defaultPeerPort = 28547
)

type config struct {
	PeerPort int
}

func loadConfig() (*config, error) {
	return &config{
		PeerPort: defaultPeerPort,
	}, nil
}
