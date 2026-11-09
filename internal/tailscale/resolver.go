package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
	"time"
)

type Device struct {
	ID        string
	Hostname  string
	LoginName string
	Tagged    bool
}

type cachedDevice struct {
	device  Device
	expires time.Time
}

// Resolver is owned by the sequential event loop.
type Resolver struct {
	binary  string
	timeout time.Duration
	ttl     time.Duration
	cache   map[netip.Addr]cachedDevice
}

func NewResolver(binary string, timeout, ttl time.Duration) *Resolver {
	return &Resolver{binary: binary, timeout: timeout, ttl: ttl, cache: make(map[netip.Addr]cachedDevice)}
}

func (r *Resolver) Resolve(ctx context.Context, ip netip.Addr) (*Device, error) {
	if cached, ok := r.cache[ip]; ok && time.Now().Before(cached.expires) {
		return &cached.device, nil
	}
	lookup, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	cmd := exec.CommandContext(lookup, r.binary, "whois", "--json", ip.String())
	cmd.WaitDelay = time.Second
	data, err := cmd.Output()
	if err != nil {
		if lookup.Err() != nil {
			return nil, lookup.Err()
		}
		return nil, fmt.Errorf("tailscale whois: %w", err)
	}
	device, err := parseWhois(data)
	if err != nil {
		return nil, err
	}
	if len(r.cache) >= 1024 {
		clear(r.cache)
	}
	r.cache[ip] = cachedDevice{device: *device, expires: time.Now().Add(r.ttl)}
	return device, nil
}

func parseWhois(data []byte) (*Device, error) {
	var reply struct {
		Node struct {
			StableID string
			Name     string
			Tags     []string
		}
		UserProfile struct{ LoginName string }
	}
	if err := json.Unmarshal(data, &reply); err != nil {
		return nil, fmt.Errorf("decode whois: %w", err)
	}
	if reply.Node.StableID == "" {
		return nil, errors.New("whois returned no stable node ID")
	}
	tagged := len(reply.Node.Tags) > 0
	if !tagged && reply.UserProfile.LoginName == "" {
		return nil, errors.New("whois returned no user identity")
	}
	return &Device{
		ID:        reply.Node.StableID,
		Hostname:  strings.TrimSuffix(reply.Node.Name, "."),
		LoginName: reply.UserProfile.LoginName,
		Tagged:    tagged,
	}, nil
}
