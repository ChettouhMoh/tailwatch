package events

import (
    "time"
    "github.com/google/uuid"
)

type EventType string

type Severity string

const (
    EventTypeSSHStarted      EventType = "SSH_SESSION_STARTED"
    EventTypeSSHEnded        EventType = "SSH_SESSION_ENDED"
    EventTypeSSHAttemptBlock EventType = "SSH_ATTEMPT_BLOCKED"
)

const (
    SeverityNormal   Severity = "NORMAL"
    SeverityCritical Severity = "CRITICAL"
)

type TrustClassification string

const (
    TrustClassificationTrusted           TrustClassification = "TRUSTED"
    TrustClassificationUnrecognizedTailnet TrustClassification = "UNRECOGNIZED_TAILNET"
    TrustClassificationExternal          TrustClassification = "EXTERNAL"
    TrustClassificationUnknown           TrustClassification = "UNKNOWN"
)

type Event struct {
    ID                    string            `json:"id"`
    Timestamp             time.Time         `json:"timestamp"`
    EventType             EventType         `json:"event_type"`
    SourceIP              string            `json:"source_ip"`
    SSHUsername           string            `json:"ssh_username"`
    DeviceHostname        string            `json:"device_hostname"`
    TailscaleDeviceID     string            `json:"tailscale_device_id"`
    TailscaleUserIdentity string            `json:"tailscale_user_identity"`
    AuthenticationInfo    string            `json:"authentication_info,omitempty"`
    TrustClassification   TrustClassification `json:"trust_classification"`
    Severity              Severity          `json:"severity"`
    DiagnosticDetails     map[string]string `json:"diagnostic_details,omitempty"`
}

func NewEvent(eventType EventType, sourceIP, sshUsername string) *Event {
    return &Event{
        ID:          uuid.New().String(),
        Timestamp:   time.Now().UTC(),
        EventType:   eventType,
        SourceIP:    sourceIP,
        SSHUsername: sshUsername,
    }
}
