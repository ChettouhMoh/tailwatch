package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("TAILWATCH_WEBHOOK_URL", "https://example.invalid/hooks/secret")
	data := `{"tailscale":{"binary":"/usr/bin/tailscale","lookup_timeout_seconds":3},"trusted_users":["me"],"trusted_device_ids":["n123"],"trusted_ips":["::ffff:100.100.1.2"],"notification":{"provider":"webhook","webhook_url":""},"monitor":{"identity_cache_seconds":60}}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TrustedIPs[0].String() != "100.100.1.2" || cfg.Notification.WebhookURL != os.Getenv("TAILWATCH_WEBHOOK_URL") {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	t.Setenv("TAILWATCH_WEBHOOK_URL", "")
	path := filepath.Join(t.TempDir(), "config.json")
	for _, data := range []string{"null", "{} {}", `{"trusted_user":["typo"]}`, `{"trusted_ips":["100.100.1.999"]}`, `{"trusted_device_ids":[""]}`,
		`{"notification":{"provider":"chat"}}`, `{"notification":{"provider":"webhook","webhook_url":"http://host/secret"}}`,
		`{"notification":{"provider":"telegram","telegram":{}}}`,
		`{"notification":{"provider":"whatsapp","whatsapp":{"phone_number_id":"123","recipient":"../fake"}}}`,
		`{"notification":{"provider":"whatsapp","whatsapp":{"phone_number_id":"123","recipient":"123","api_version":"x/123","template":"login","language":"en_US"}}}`,
		`{"tailscale":{"lookup_timeout_seconds":-3}}`, `{"monitor":{"identity_cache_seconds":9223372036854775807}}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("config %s got error %v", data, err)
		}
	}
}
