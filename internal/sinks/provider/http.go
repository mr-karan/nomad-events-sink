package provider

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPManager represents the various methods for interacting with Pigeon.
type HTTPManager struct {
	client  http.Client
	rootURL string
	log     *logrus.Logger

	healthCheckURL    string
	healthCheckStatus int
}

type HTTPOpts struct {
	Log            *logrus.Logger
	RootURL        string
	Timeout        time.Duration
	MaxConnections int

	HealthCheckEnabled bool
	HealthcheckURL     string
	HealthCheckStatus  int
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
		client:  client,
		rootURL: opts.RootURL,
		log:     opts.Log,
	}

	// HeathlCheck upstream if healthcheck is enabled.
	if opts.HealthCheckEnabled {
		httpMgr.healthCheckURL = opts.HealthcheckURL
		httpMgr.healthCheckStatus = opts.HealthCheckStatus
		return httpMgr, httpMgr.HealthCheck()
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
func (m *HTTPManager) Name() string {
	return "http"
}

// HealthCheck does a simple HTTP GET request to the
func (m HTTPManager) HealthCheck() error {
	url, status := m.healthCheckURL, m.healthCheckStatus

	m.log.WithField("provider", m.Name()).WithField("url", url).Info("attempting to check provider health")

	resp, err := m.client.Get(url)
	if err != nil {
		return err
	}

	if resp.StatusCode != status {
		return fmt.Errorf("status mismatch; expected %d got %d - %s", status, resp.StatusCode, resp.Status)
	}

	return nil
}
