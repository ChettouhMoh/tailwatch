package monitor

import (
	"net/netip"
	"testing"

	"tailwatch/internal/config"
	"tailwatch/internal/events"
	"tailwatch/internal/tailscale"
)

func TestClassify(t *testing.T) {
	owner := "owner@example.com"
	cfg := &config.Config{TrustedUsers: []string{owner}, TrustedDeviceIDs: []string{"n-known"}, TrustedIPs: []netip.Addr{netip.MustParseAddr("100.70.1.1")}}
	known := &tailscale.Device{ID: "n-known", LoginName: owner}
	cases := []struct {
		name, ip, user string
		device         *tailscale.Device
		want           events.TrustClassification
	}{
		{"explicit IP", "100.70.1.1", owner, nil, events.Trusted},
		{"owner allowlist", "100.100.0.1", owner, &tailscale.Device{ID: "n-new", LoginName: owner}, events.Trusted},
		{"device allowlist", "100.100.0.1", "other", &tailscale.Device{ID: "n-known", LoginName: "other"}, events.Trusted},
		{"unknown device", "100.100.0.1", "other", &tailscale.Device{ID: "n-new", LoginName: "other"}, events.UnrecognizedTailnet},
		{"tagged machine cannot inherit user trust", "100.100.0.1", owner, &tailscale.Device{ID: "n-tagged", LoginName: owner, Tagged: true}, events.UnrecognizedTailnet},
		{"changed identity overrides trusted IP and ID", "100.70.1.1", "other", known, events.Unknown},
		{"journal-only trusted user", "100.100.0.1", owner, nil, events.Unknown},
		{"CGNAT lower bound", "100.64.0.0", owner, nil, events.Unknown},
		{"CGNAT upper bound", "100.127.255.255", owner, nil, events.Unknown},
		{"above CGNAT range", "100.128.0.0", owner, nil, events.External},
		{"below CGNAT range", "100.63.255.255", owner, nil, events.External},
		{"IPv6", "fd7a:115c:a1e0::1", owner, nil, events.Unknown},
		{"external IPv6", "2001:db8::1", owner, nil, events.External},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(cfg, events.Session{SourceIP: netip.MustParseAddr(tc.ip), TailscaleUserIdentity: tc.user}, tc.device)
			severity := events.SeverityCritical
			if tc.want == events.Trusted {
				severity = events.SeverityNormal
			}
			if got.TrustClassification != tc.want || got.Severity != severity || got.Reason == "" {
				t.Fatalf("got %+v; want %s (%s)", got, tc.want, severity)
			}
		})
	}
}
