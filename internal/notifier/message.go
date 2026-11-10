package notifier

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"tailwatch/internal/events"
)

type loginMessage struct {
	events.Event
	Message string `json:"message"`
}

func envelope(e events.Event) loginMessage {
	return loginMessage{Event: e, Message: message(e)}
}

func message(e events.Event) string {
	via := "Tailscale SSH"
	if e.SSHServer == events.OpenSSH {
		via = "OpenSSH"
		if e.TailscaleDeviceID != "" {
			via = "OpenSSH (Tailscale peer)"
		}
	}
	parts := []string{fmt.Sprintf("SSH login on %s as %s via %s", field(e.Server), field(e.SSHUsername), via)}
	if e.AuthMethod != "" {
		parts = append(parts, "method "+field(e.AuthMethod))
	}
	if e.SourceIP.IsValid() {
		parts = append(parts, "from "+e.SourceIP.String())
	}
	if e.TailscaleUserIdentity != "" {
		parts = append(parts, "Tailscale login "+field(e.TailscaleUserIdentity))
	}
	if e.DeviceHostname != "" {
		parts = append(parts, "device "+field(e.DeviceHostname))
	}
	if e.PeerLogin != "" {
		parts = append(parts, "peer owner "+field(e.PeerLogin))
	}
	parts = append(parts, string(e.TrustClassification), e.Timestamp.UTC().Format(time.RFC3339))
	return strings.Join(parts, " · ")
}

// Keep externally supplied fields on one line in plain-text alerts, including
// WhatsApp template parameters. The structured event retains the original values.
func field(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return ' '
		}
		return r
	}, value)
	runes := []rune(strings.Join(strings.Fields(value), " "))
	if len(runes) > 96 {
		return string(runes[:95]) + "…"
	}
	return string(runes)
}
