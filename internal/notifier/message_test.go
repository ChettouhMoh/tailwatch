package notifier

import (
	"net/netip"
	"strings"
	"testing"

	"tailwatch/internal/events"
)

func TestMessagesUseOnlyAvailableIdentity(t *testing.T) {
	login := events.Event{Session: events.Session{Server: "vps", SSHUsername: "deploy", SSHServer: events.TailscaleSSH, TailscaleUserIdentity: "alice@example.com"}}
	text := message(login)
	if !strings.Contains(text, "via Tailscale SSH") || !strings.Contains(text, "Tailscale login alice@example.com") || strings.Contains(text, "from ") {
		t.Fatal(text)
	}
	login.SSHServer = events.OpenSSH
	login.TailscaleUserIdentity = ""
	login.SourceIP = netip.MustParseAddr("203.0.113.4")
	login.AuthMethod = "publickey"
	text = message(login)
	if !strings.Contains(text, "via OpenSSH") || !strings.Contains(text, "from 203.0.113.4") || !strings.Contains(text, "method publickey") || strings.Contains(text, "Tailscale") {
		t.Fatal(text)
	}
	login.TailscaleDeviceID = "n1"
	login.DeviceHostname = "test\nbox"
	login.PeerLogin = "a@example.com"
	text = message(login)
	if !strings.Contains(text, "OpenSSH (Tailscale peer)") || !strings.Contains(text, "peer owner a@example.com") || !strings.Contains(text, "device test box") || strings.Contains(text, "\n") {
		t.Fatal(text)
	}
}
