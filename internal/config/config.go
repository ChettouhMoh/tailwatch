package config

import (
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
	WhatsApp WhatsAppConfig `json:"whatsapp"`
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
