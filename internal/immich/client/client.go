package client

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

// Config holds configuration options for the Immich HTTP client.
type Config struct {
	// EndPoint is the base URL of the Immich server (without /api suffix).
	EndPoint string

	// APIKey is the user's API key for authentication.
	APIKey string

	// DeviceUUID identifies the client device.
	DeviceUUID string

	// SkipSSLVerify disables SSL certificate verification.
	SkipSSLVerify bool

	// Timeout is the overall request timeout.
	Timeout time.Duration

	// ResponseHeaderTimeout is the timeout for reading response headers.
	ResponseHeaderTimeout time.Duration

	// Retries is the number of retry attempts on 5xx errors.
	Retries int

	// RetriesDelay is the duration between retries.
	RetriesDelay time.Duration

	// DryRun prevents any write operations when true.
	DryRun bool

	// TraceWriter enables API tracing when non-nil.
	TraceWriter io.Writer
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	deviceUUID, _ := os.Hostname()
	return Config{
		DeviceUUID:            deviceUUID,
		SkipSSLVerify:         false,
		Timeout:               20 * time.Minute,
		ResponseHeaderTimeout: 20 * time.Minute,
		Retries:               1,
		RetriesDelay:          time.Second,
		DryRun:                false,
	}
}

// Option is a functional option for configuring the HTTP client.
type Option func(*Config)

// WithEndPoint sets the server endpoint.
func WithEndPoint(endpoint string) Option {
	return func(c *Config) {
		c.EndPoint = endpoint
	}
}

// WithAPIKey sets the API key.
func WithAPIKey(key string) Option {
	return func(c *Config) {
		c.APIKey = key
	}
}

// WithDeviceUUID sets the device UUID.
func WithDeviceUUID(uuid string) Option {
	return func(c *Config) {
		c.DeviceUUID = uuid
	}
}

// WithSkipSSLVerify configures SSL verification.
func WithSkipSSLVerify(skip bool) Option {
	return func(c *Config) {
		c.SkipSSLVerify = skip
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithRetries sets the retry configuration.
func WithRetries(count int, delay time.Duration) Option {
	return func(c *Config) {
		c.Retries = count
		c.RetriesDelay = delay
	}
}

// WithDryRun enables dry run mode.
func WithDryRun(dryRun bool) Option {
	return func(c *Config) {
		c.DryRun = dryRun
	}
}

// WithTraceWriter enables API tracing.
func WithTraceWriter(w io.Writer) Option {
	return func(c *Config) {
		c.TraceWriter = w
	}
}

// HTTPClient wraps an http.Client with Immich-specific configuration.
type HTTPClient struct {
	*http.Client
	transport *http.Transport
	config    Config
}

// NewHTTPClient creates a new HTTP client with the given options.
func NewHTTPClient(opts ...Option) *HTTPClient {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: cfg.SkipSSLVerify},
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
	}

	return &HTTPClient{
		Client: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
		transport: transport,
		config:    cfg,
	}
}

// Config returns the current configuration.
func (c *HTTPClient) Config() Config {
	return c.config
}

// EndPoint returns the API endpoint URL.
func (c *HTTPClient) EndPoint() string {
	return c.config.EndPoint + "/api"
}

// APIKey returns the configured API key.
func (c *HTTPClient) APIKey() string {
	return c.config.APIKey
}

// DeviceUUID returns the device UUID.
func (c *HTTPClient) DeviceUUID() string {
	return c.config.DeviceUUID
}

// DryRun returns whether dry run mode is enabled.
func (c *HTTPClient) DryRun() bool {
	return c.config.DryRun
}

// RoundTripperDecorator wraps an http.RoundTripper for tracing/logging.
type RoundTripperDecorator func(rt http.RoundTripper) http.RoundTripper

// EnableTrace decorates the transport with the given decorator.
func (c *HTTPClient) EnableTrace(rtd RoundTripperDecorator) {
	if rtd != nil {
		c.Client.Transport = rtd(c.Client.Transport)
	} else {
		c.Client.Transport = c.transport
	}
}
