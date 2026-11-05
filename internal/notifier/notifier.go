package notifier

import (
    "context"
)

type Event struct {
    ID                    string
    Timestamp             string
    EventType             string
    SourceIP              string
    SSHUsername           string
    DeviceHostname        string
    TailscaleDeviceID     string
    TailscaleUserIdentity string
    TrustClassification   string
    Severity              string
}

type Notifier interface {
    Notify(ctx context.Context, event Event) error
}

func NewEventFromInternal(event interface{}) Event {
    // Convert from internal event format
    // This is a simplified implementation
    return Event{}
}
