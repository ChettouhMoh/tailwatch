package config

type TailscaleConfig struct {
    Binary            string `json:"binary"`
    LookupTimeoutSeconds int `json:"lookup_timeout_seconds"`
}

type NotificationConfig struct {
    Provider    string `json:"provider"`
    WebhookURL  string `json:"webhook_url"`
}

type MonitorConfig struct {
    IdentityCacheSeconds int `json:"identity_cache_seconds"`
}

type Config struct {
    Tailscale     TailscaleConfig     `json:"tailscale"`
    TrustedUsers  []string           `json:"trusted_users"`
    TrustedDeviceIDs []string         `json:"trusted_device_ids"`
    TrustedIPs    []string           `json:"trusted_ips"`
    Notification  NotificationConfig `json:"notification"`
    Monitor       MonitorConfig      `json:"monitor"`
}

func (c *Config) Validate() error {
    // Validate tailscale binary exists (check at runtime)
    if c.Tailscale.Binary == "" {
        c.Tailscale.Binary = "/usr/bin/tailscale" // default
    }
    
    if c.Tailscale.LookupTimeoutSeconds <= 0 {
        c.Tailscale.LookupTimeoutSeconds = 3
    }
    
    if c.Monitor.IdentityCacheSeconds <= 0 {
        c.Monitor.IdentityCacheSeconds = 60
    }
    
    return nil
}
