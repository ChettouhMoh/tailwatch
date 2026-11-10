package events

import (
	"net/netip"
	"time"
)

type Severity string
type TrustClassification string
type SSHServer string

const (
	TailscaleSSH SSHServer = "tailscale"
	OpenSSH      SSHServer = "openssh"
)

const (
	SeverityNormal   Severity = "NORMAL"
	SeverityCritical Severity = "CRITICAL"

	Trusted             TrustClassification = "TRUSTED"
	UnrecognizedTailnet TrustClassification = "UNRECOGNIZED_TAILNET"
	External            TrustClassification = "EXTERNAL"
	Unknown             TrustClassification = "UNKNOWN"
)

// Session contains the login as observed by the authenticating SSH server.
type Session struct {
	ID                    string     `json:"id"`
	Timestamp             time.Time  `json:"timestamp"`
	Server                string     `json:"server"`
	SSHServer             SSHServer  `json:"ssh_server"`
	AuthMethod            string     `json:"auth_method,omitempty"`
	SourceIP              netip.Addr `json:"source_ip"`
	SSHUsername           string     `json:"ssh_username"`
	TailscaleUserIdentity string     `json:"tailscale_user_identity"`
}

type Decision struct {
	TrustClassification TrustClassification `json:"trust_classification"`
	Severity            Severity            `json:"severity"`
	Reason              string              `json:"reason"`
}

type Event struct {
	Session
	Decision
	EventType         string `json:"event_type"`
	DeviceHostname    string `json:"device_hostname,omitempty"`
	TailscaleDeviceID string `json:"tailscale_device_id,omitempty"`
	PeerLogin         string `json:"tailscale_peer_login,omitempty"`
}
