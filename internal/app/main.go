package app

import (
    "context"
    "fmt"
    "log/slog"
    "time"
)

import "tailwatch/internal/events"
import "tailwatch/internal/monitor"
import "tailwatch/internal/notifier"
import "tailwatch/internal/source"
import "tailwatch/internal/tailscale"

type App struct {
    config          Config
    eventSource     source.EventSource
    identityResolver *tailscale.IdentityResolver
    classifier      *monitor.DeviceClassifier
    notifiers       []notifier.Notifier
    logger          *slog.Logger
}

type Config interface {
    GetTrustedUsers() []string
    GetTrustedDeviceIDs() []string
    GetTrustedIPs() []string
    GetTailscaleBinary() string
    GetTailscaleTimeout() int
    GetNotificationProviders() []string
}

type RealConfig struct {
    trustedUsers     []string
    trustedDeviceIDs []string
    trustedIPs       []string
    tailscaleBinary  string
    tailscaleTimeout int
    notificationProviders []string
}

func NewRealConfig(appConfig interface{}) *RealConfig {
    // Convert from internal config format
    // This will be implemented with actual config loading
    return &RealConfig{
        trustedUsers:     []string{"test@example.com"},
        trustedDeviceIDs: []string{},
        trustedIPs:       []string{},
        tailscaleBinary:  "/usr/bin/tailscale",
        tailscaleTimeout: 3,
        notificationProviders: []string{"stdout"},
    }
}

func (c *RealConfig) GetTrustedUsers() []string { return c.trustedUsers }
func (c *RealConfig) GetTrustedDeviceIDs() []string { return c.trustedDeviceIDs }
func (c *RealConfig) GetTrustedIPs() []string { return c.trustedIPs }
func (c *RealConfig) GetTailscaleBinary() string { return c.tailscaleBinary }
func (c *RealConfig) GetTailscaleTimeout() int { return c.tailscaleTimeout }
func (c *RealConfig) GetNotificationProviders() []string { return c.notificationProviders }

func NewApp(appConfig interface{}, logger *slog.Logger) (*App, error) {
    // For now, use test event source for development
    // In production, this would use a real event source when available
    eventSource := &source.TestEventSource{}
    
    config := NewRealConfig(appConfig)
    
    identityResolver := tailscale.NewIdentityResolver(
        config.GetTailscaleBinary(),
        config.GetTailscaleTimeout(),
        60, // cache expiry
    )
    
    classifier := monitor.NewDeviceClassifier(config)
    
    // Create notifiers
    var notifiers []notifier.Notifier
    // Note: In production, you'd set up real notifiers here
    
    return &App{
        config:          config,
        eventSource:     eventSource,
        identityResolver: identityResolver,
        classifier:      classifier,
        notifiers:       notifiers,
        logger:          logger,
    }, nil
}

func (a *App) Start(ctx context.Context) error {
    // Start event source
    if err := a.eventSource.Start(ctx); err != nil {
        return fmt.Errorf("failed to start event source: %w", err)
    }
    
    // Get events from test source
    testEvents := a.eventSource.GetTestEvents()
    
    // Process each event
    for _, testEvent := range testEvents {
        if err := a.processEvent(ctx, testEvent); err != nil {
            a.logger.Error("Error processing event", "error", err)
        }
    }
    
    return nil
}

func (a *App) processEvent(ctx context.Context, testEvent source.TestEvent) error {
    // Resolve Tailscale identity
    deviceInfo, err := a.identityResolver.Resolve(testEvent.SourceIP)
    if err != nil {
        a.logger.Error("Identity resolution error", "error", err, "ip", testEvent.SourceIP)
    }
    
    // Determine trust classification
    classification := a.classifier.Classify(testEvent.SourceIP, deviceInfo)
    
    // Create event
    event := events.NewEvent(
        events.EventTypeSSHStarted,
        testEvent.SourceIP,
        testEvent.SSHUsername,
    )
    
    event.DeviceHostname = testEvent.DeviceHostname
    event.TailscaleDeviceID = deviceInfo.ID
    event.TailscaleUserIdentity = deviceInfo.UserEmail
    event.TrustClassification = classification.Classification
    event.Severity = classification.Severity
    
    // Set authentication info if available
    if deviceInfo != nil {
        event.AuthenticationInfo = fmt.Sprintf("Tailscale user: %s, Device: %s", 
            deviceInfo.UserEmail, deviceInfo.Hostname)
    }
    
    // Notify via notifiers
    for _, notifier := range a.notifiers {
        if err := notifier.Notify(ctx, event); err != nil {
            a.logger.Error("Notification error", "error", err)
        }
    }
    
    return nil
}
