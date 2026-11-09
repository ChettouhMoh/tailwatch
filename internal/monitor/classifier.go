package monitor

import (
	"net/netip"
	"slices"

	"tailwatch/internal/config"
	"tailwatch/internal/events"
	"tailwatch/internal/tailscale"
)

var (
	tailnetIPv4 = netip.MustParsePrefix("100.64.0.0/10")
	tailnetIPv6 = netip.MustParsePrefix("fd7a:115c:a1e0::/48")
)

func Classify(cfg *config.Config, session events.Session, device *tailscale.Device) events.Decision {
	ip := session.SourceIP.Unmap()
	switch {
	case !ip.IsValid():
		return decision(events.Unknown, "Session was granted but the journal record contains no source IP")
	case device != nil && device.LoginName != "" && device.LoginName != session.TailscaleUserIdentity:
		return decision(events.Unknown, "Current device identity differs from the user in the session log")
	case slices.Contains(cfg.TrustedIPs, ip):
		return decision(events.Trusted, "Source IP is allowlisted")
	case device != nil && slices.Contains(cfg.TrustedDeviceIDs, device.ID):
		return decision(events.Trusted, "Device ID is allowlisted")
	case device != nil && !device.Tagged && slices.Contains(cfg.TrustedUsers, device.LoginName):
		return decision(events.Trusted, "Tailscale user is allowlisted")
	case device != nil:
		return decision(events.UnrecognizedTailnet, "Resolved device and user have no matching allowlist entry")
	case tailnetIPv4.Contains(ip) || tailnetIPv6.Contains(ip):
		return decision(events.Unknown, "Could not resolve the tailnet address to a device")
	default:
		return decision(events.External, "Unresolved source is outside the Tailscale address ranges")
	}
}

func decision(class events.TrustClassification, reason string) events.Decision {
	severity := events.SeverityCritical
	if class == events.Trusted {
		severity = events.SeverityNormal
	}
	return events.Decision{TrustClassification: class, Severity: severity, Reason: reason}
}
