package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"regexp"
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
	Telegram   struct {
		ChatID string `json:"chat_id"`
	} `json:"telegram"`
	WhatsApp   WhatsAppConfig `json:"whatsapp"`
	credential *string
}

// A managed channel supplies its own credential. Legacy setups use the environment.
func (n NotificationConfig) Credential(envName string) string {
	if n.credential != nil {
		return *n.credential
	}
	return os.Getenv(envName)
}

type WhatsAppConfig struct {
	PhoneNumberID string `json:"phone_number_id"`
	Recipient     string `json:"recipient"`
	Template      string `json:"template"`
	Language      string `json:"language"`
	APIVersion    string `json:"api_version"`
}

var (
	digits       = regexp.MustCompile(`^[0-9]+$`)
	graphVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+$`)
)

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
	return LoadWithChannel(path, "")
}

// LoadWithChannel applies the optional systemd credential before validating the
// effective configuration. A fresh package supplies an empty file for legacy installs.
func LoadWithChannel(path, channelPath string) (*Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := decodeObject(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if override := os.Getenv("TAILWATCH_WEBHOOK_URL"); override != "" {
		cfg.Notification.WebhookURL = override
	}
	var channel struct {
		NotificationConfig
		Secret string `json:"credential"`
	}
	if channelPath != "" {
		// An empty credential leaves the legacy settings active.
		data, err := os.ReadFile(channelPath)
		if err != nil {
			return nil, fmt.Errorf("read channel: %w", err)
		}
		if len(data) > 0 {
			if err := decodeObject(data, &channel); err != nil || channel.Provider == "" {
				return nil, errors.New("invalid channel configuration: expected a JSON object with a provider")
			}
			cfg.Notification = channel.NotificationConfig
			cfg.Notification.credential = &channel.Secret
			if channel.Provider == "webhook" {
				cfg.Notification.WebhookURL = channel.Secret
			}
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func decodeObject(data []byte, dest any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("expected one JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	if decoder.Decode(new(json.RawMessage)) != io.EOF {
		return errors.New("expected one JSON object")
	}
	return nil
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
	case "telegram":
		if strings.TrimSpace(c.Notification.Telegram.ChatID) == "" {
			return errors.New("telegram requires notification.telegram.chat_id")
		}
	case "whatsapp":
		w := c.Notification.WhatsApp
		if !digits.MatchString(w.PhoneNumberID) || !digits.MatchString(w.Recipient) {
			return errors.New("whatsapp requires numeric phone_number_id and recipient including country code")
		}
		if !graphVersion.MatchString(w.APIVersion) || strings.TrimSpace(w.Template) == "" || strings.TrimSpace(w.Language) == "" {
			return errors.New("whatsapp requires api_version (vN.N), template and language")
		}
	default:
		return fmt.Errorf("unknown notification provider %q", c.Notification.Provider)
	}
	return nil
}
