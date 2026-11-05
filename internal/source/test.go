package source

import (
    "context"
)

type EventSource interface {
    Start(ctx context.Context) error
    Stop() error
    GetTestEvents() []TestEvent
}

type TestEventSource struct {
}

func (s *TestEventSource) Start(ctx context.Context) error {
    return nil
}

func (s *TestEventSource) Stop() error {
    return nil
}

func (s *TestEventSource) GetTestEvents() []TestEvent {
    return []TestEvent{
        {
            SourceIP:       "100.64.0.1",
            SSHUsername:    "mohamed",
            DeviceHostname: "mohamed-laptop",
            TailscaleUser:  "mohamed@example.com",
            EventType:      "SSH_SESSION_STARTED",
        },
        {
            SourceIP:       "100.64.0.2",
            SSHUsername:    "unknown",
            DeviceHostname: "unknown-device",
            TailscaleUser:  "",
            EventType:      "SSH_SESSION_STARTED",
        },
    }
}

type TestEvent struct {
    SourceIP        string
    SSHUsername     string
    DeviceHostname  string
    TailscaleUser   string
    EventType       string
}
