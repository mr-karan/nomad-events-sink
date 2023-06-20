package provider

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/knadh/koanf"
	"github.com/sirupsen/logrus"
)

// HTTPManager represents the various methods for interacting with Pigeon.
type HTTPManager struct {
	name    string
	client  http.Client
	rootURL string
	log     *logrus.Logger
}

type HTTPOpts struct {
	Log            *logrus.Logger
	Name           string
	RootURL        string
	Timeout        time.Duration
	MaxConnections int

	HealthCheckEnabled bool
	HealthcheckURL     string
	HealthCheckStatus  int
}

func ParseHTTPOpts(name string, ko *koanf.Koanf) HTTPOpts {
	return HTTPOpts{
		Name:               name,
		RootURL:            ko.String("root_url"),
		Timeout:            ko.Duration("timeout"),
		MaxConnections:     ko.Int("max_idle_conns"),
		HealthCheckEnabled: ko.Bool("healthcheck.enabled"),
		HealthcheckURL:     ko.String("healthcheck.url"),
		HealthCheckStatus:  ko.Int("healthcheck.status"),
	}
}

// NewHTTP initializes a HTTP notification dispatcher object.
func NewHTTP(opts HTTPOpts) (*HTTPManager, error) {
	if opts.RootURL == "" {
		return nil, fmt.Errorf("sink HTTP provider misconfigured. Missing required value: `root_url`")
	}
	if opts.Timeout < time.Duration(1)*time.Second {
		return nil, fmt.Errorf("sink HTTP provider misconfigured. `timeout` value is dangerously low: %s", opts.Timeout)
	}
	if opts.MaxConnections == 0 {
		opts.MaxConnections = 100
	}

	// Initialise HTTP Client object.
	client := http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   opts.MaxConnections,
			ResponseHeaderTimeout: opts.Timeout,
		},
	}

	httpMgr := &HTTPManager{
		name:    opts.Name,
		client:  client,
		rootURL: opts.RootURL,
		log:     opts.Log,
	}

	// Ping upstream if healthcheck is enabled.
	if opts.HealthCheckEnabled {
		httpMgr.log.WithField("url", opts.HealthcheckURL).Info("attempting to ping provider")
		return httpMgr, httpMgr.Ping(opts.HealthcheckURL, opts.HealthCheckStatus)
	}

	return httpMgr, nil
}

// Push sends out events to an HTTP Endpoint.
func (m *HTTPManager) Push(data []byte) error {
	req, err := http.NewRequest("POST", m.rootURL, bytes.NewBuffer(data))
	if err != nil {
		m.log.WithError(err).Error("error preparing http request")
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		m.log.WithError(err).Error("error sending http request")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non 200 OK from upstream: %s", resp.Status)
	}

	return nil
}

// Name returns the notification provider name.
func (m HTTPManager) Name() string {
	return "http." + m.name
}

// Ping does a simple HTTP GET request to the
func (m *HTTPManager) Ping(url string, status int) error {
	resp, err := m.client.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != status {
		return fmt.Errorf("status mismatch; expected %d got %d - %s", status, resp.StatusCode, resp.Status)
	}
	return nil
}
