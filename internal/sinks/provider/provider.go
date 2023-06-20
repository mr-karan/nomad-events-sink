package provider

type Provider interface {
	// Name returns the name of the Provider.
	Name() string
	// Push pushes a batch of event to upstream. The implementation varies across providers.
	Push([]byte) error
	// HealthCheck implements a healthcheck.
	HealthCheck() error
}
