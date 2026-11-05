package tailscale

import (
    "bytes"
    "encoding/json"
    "os"
    "os/exec"
    "time"
)

type TailscaleDeviceInfo struct {
    ID        string   `json:"id"`
    Hostname  string   `json:"hostname"`
    UserEmail string   `json:"userEmail"`
    Addresses []string `json:"addresses"`
}

type TailscaleStatus struct {
    Peers []struct {
        ID       string   `json:"id"`
        Hostname string   `json:"hostname"`
        User     string   `json:"user"`
    } `json:"peers"`
}

type IdentityResolver struct {
    binaryPath      string
    timeoutSeconds  int
    cache           map[string]*CachedInfo
    cacheExpiry     time.Duration
}

type CachedInfo struct {
    DeviceInfo *TailscaleDeviceInfo
    Timestamp  time.Time
}

func NewIdentityResolver(binaryPath string, timeoutSeconds int, cacheExpirySeconds int) *IdentityResolver {
    return &IdentityResolver{
        binaryPath:     binaryPath,
        timeoutSeconds: timeoutSeconds,
        cache:          make(map[string]*CachedInfo),
        cacheExpiry:    time.Duration(cacheExpirySeconds) * time.Second,
    }
}

func (r *IdentityResolver) Resolve(sourceIP string) (*TailscaleDeviceInfo, error) {
    // Check cache first
    if cached, exists := r.cache[sourceIP]; exists {
        if time.Since(cached.Timestamp) < r.cacheExpiry {
            return cached.DeviceInfo, nil
        }
    }

    // Use tailscale whois --json
    cmd := exec.Command(r.binaryPath, "whois", "--json", sourceIP)

    var out bytes.Buffer
    cmd.Stdout = &out

    if err := cmd.Run(); err != nil {
        // If tailscale is not available or fails, return no identity
        // This is a critical requirement: never crash the daemon
        return nil, nil
    }

    // Try to parse the output
    if out.Len() > 0 {
        var deviceInfo TailscaleDeviceInfo
        if err := json.Unmarshal(out.Bytes(), &deviceInfo); err != nil {
            // If we can't parse, treat as unknown identity
            return nil, nil
        }

        // Cache the result
        r.cache[sourceIP] = &CachedInfo{
            DeviceInfo: &deviceInfo,
            Timestamp:  time.Now(),
        }

        return &deviceInfo, nil
    }

    return nil, nil
}

func (r *IdentityResolver) IsAvailable() bool {
    // Check if tailscale binary exists
    _, err := os.Stat(r.binaryPath)
    return err == nil
}
