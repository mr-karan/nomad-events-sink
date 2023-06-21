package provider

import "github.com/hashicorp/nomad/api"

type Provider interface {
	// Name returns the name of the Provider.
	Name() string
	// Prepare returns a prepared payload as an array of bytes
	Prepare([]api.Event) ([]byte, error)
	// Push pushes a batch of event to upstream. The implementation varies across providers.
	Push([]byte) error
	// Ping implements a healthcheck.
	Ping(url string, status int) error
}
