package monitor

import "tailwatch/internal/events"
import "tailwatch/internal/tailscale"

type Config interface {
    GetTrustedUsers() []string
    GetTrustedDeviceIDs() []string
    GetTrustedIPs() []string
}

type DeviceClassifier struct {
    trustedUsers      []string
    trustedDeviceIDs  []string
    trustedIPs        []string
}

type ClassificationResult struct {
    Classification events.TrustClassification
    Severity       events.Severity
    Reason         string
}

func NewDeviceClassifier(config Config) *DeviceClassifier {
    return &DeviceClassifier{
        trustedUsers:     config.GetTrustedUsers(),
        trustedDeviceIDs: config.GetTrustedDeviceIDs(),
        trustedIPs:       config.GetTrustedIPs(),
    }
}

func (c *DeviceClassifier) Classify(sourceIP string, deviceInfo *tailscale.TailscaleDeviceInfo) ClassificationResult {
    // First, check if source IP is in trusted IPs list
    for _, trustedIP := range c.trustedIPs {
        if trustedIP == sourceIP {
            return ClassificationResult{
                Classification: events.TrustClassificationTrusted,
                Severity:       events.SeverityNormal,
                Reason:         "Source IP is allowlisted",
            }
        }
    }

    // Check if Tailscale identity resolved
    if deviceInfo != nil {
        // Check device ID
        for _, trustedID := range c.trustedDeviceIDs {
            if trustedID == deviceInfo.ID {
                return ClassificationResult{
                    Classification: events.TrustClassificationTrusted,
                    Severity:       events.SeverityNormal,
                    Reason:         "Device ID is allowlisted",
                }
            }
        }

        // Check user email
        for _, trustedUser := range c.trustedUsers {
            if trustedUser == deviceInfo.UserEmail {
                return ClassificationResult{
                    Classification: events.TrustClassificationTrusted,
                    Severity:       events.SeverityNormal,
                    Reason:         "User email is allowlisted",
                }
            }
        }

        // Device/user is recognized but not allowlisted
        return ClassificationResult{
            Classification: events.TrustClassificationUnrecognizedTailnet,
            Severity:       events.SeverityCritical,
            Reason:         "Device/user recognized but not allowlisted",
        }
    }

    // Check if IP is in Tailscale 100.64.0.0/10 range
    if isTailscaleIP(sourceIP) {
        // Tailscale IP but identity resolution failed
        return ClassificationResult{
            Classification: events.TrustClassificationUnknown,
            Severity:       events.SeverityCritical,
            Reason:         "Tailscale IP but identity resolution failed",
        }
    }

    // External IP (not Tailscale)
    return ClassificationResult{
        Classification: events.TrustClassificationExternal,
        Severity:       events.SeverityCritical,
        Reason:         "Source is external (not Tailscale)",
    }
}

func isTailscaleIP(ip string) bool {
    // Check if IP is in Tailscale's unique IP range (100.64.0.0/10)
    // This is the carrier-grade NAT range used by Tailscale
    return len(ip) >= 7 && ip[:7] == "100.64."
}
