package client

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Timeout != 20*time.Minute {
		t.Errorf("expected timeout 20m, got %v", cfg.Timeout)
	}
	if cfg.Retries != 1 {
		t.Errorf("expected 1 retry, got %d", cfg.Retries)
	}
	if cfg.DryRun {
		t.Error("expected DryRun to be false")
	}
}

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient(
		WithEndPoint("http://localhost:2283"),
		WithAPIKey("test-key"),
		WithDeviceUUID("test-device"),
		WithSkipSSLVerify(true),
		WithTimeout(5*time.Minute),
		WithRetries(3, 2*time.Second),
		WithDryRun(true),
	)

	if client.EndPoint() != "http://localhost:2283/api" {
		t.Errorf("expected endpoint 'http://localhost:2283/api', got %s", client.EndPoint())
	}
	if client.APIKey() != "test-key" {
		t.Errorf("expected API key 'test-key', got %s", client.APIKey())
	}
	if client.DeviceUUID() != "test-device" {
		t.Errorf("expected device UUID 'test-device', got %s", client.DeviceUUID())
	}
	if !client.DryRun() {
		t.Error("expected DryRun to be true")
	}

	cfg := client.Config()
	if cfg.Retries != 3 {
		t.Errorf("expected 3 retries, got %d", cfg.Retries)
	}
	if cfg.Timeout != 5*time.Minute {
		t.Errorf("expected timeout 5m, got %v", cfg.Timeout)
	}
}

func TestHTTPClientDefaults(t *testing.T) {
	client := NewHTTPClient()

	if client.Client == nil {
		t.Error("expected non-nil http.Client")
	}
	if client.transport == nil {
		t.Error("expected non-nil transport")
	}
}
