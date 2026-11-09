package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

type TailscaleConfig struct {
	Binary               string `json:"binary"`
	LookupTimeoutSeconds int64  `json:"lookup_timeout_seconds"`
}

type NotificationConfig struct {
	Provider   string `json:"provider"`
	WebhookURL string `json:"webhook_url"`
}

type MonitorConfig struct {
	IdentityCacheSeconds int64 `json:"identity_cache_seconds"`
}

type Config struct {
	Tailscale        TailscaleConfig    `json:"tailscale"`
	TrustedUsers     []string           `json:"trusted_users"`
	TrustedDeviceIDs []string           `json:"trusted_device_ids"`
	TrustedIPs       []netip.Addr       `json:"trusted_ips"`
	Notification     NotificationConfig `json:"notification"`
	Monitor          MonitorConfig      `json:"monitor"`
}

func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg *Config
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if cfg == nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
		return nil, errors.New("config must contain one JSON object")
	}
	if override := os.Getenv("TAILWATCH_WEBHOOK_URL"); override != "" {
		cfg.Notification.WebhookURL = override
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Tailscale.Binary == "" {
		c.Tailscale.Binary = "tailscale"
	}
	if c.Tailscale.LookupTimeoutSeconds == 0 {
		c.Tailscale.LookupTimeoutSeconds = 3
	}
	if c.Monitor.IdentityCacheSeconds == 0 {
		c.Monitor.IdentityCacheSeconds = 60
	}
	const maxSeconds = int64((1<<63 - 1) / time.Second)
	for _, seconds := range []int64{c.Tailscale.LookupTimeoutSeconds, c.Monitor.IdentityCacheSeconds} {
		if seconds < 0 || seconds > maxSeconds {
			return errors.New("timeout and cache duration must be valid positive seconds")
		}
	}
	for _, list := range [][]string{c.TrustedUsers, c.TrustedDeviceIDs} {
		for _, value := range list {
			if value == "" || strings.TrimSpace(value) != value {
				return errors.New("trusted_users and trusted_device_ids require nonempty values without surrounding whitespace")
			}
		}
	}
	for i, ip := range c.TrustedIPs {
		if !ip.IsValid() || ip.Zone() != "" {
			return fmt.Errorf("invalid trusted_ips entry at index %d", i)
		}
		c.TrustedIPs[i] = ip.Unmap()
	}
	switch c.Notification.Provider {
	case "", "stdout":
		c.Notification.Provider = "stdout"
	case "webhook":
		u, err := url.Parse(c.Notification.WebhookURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return errors.New("webhook provider requires an HTTPS URL without embedded credentials or a fragment")
		}
	default:
		return fmt.Errorf("unknown notification provider %q", c.Notification.Provider)
	}
	return nil
}
