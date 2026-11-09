package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"slices"
	"time"

	"tailwatch/internal/config"
	"tailwatch/internal/events"
	"tailwatch/internal/monitor"
	"tailwatch/internal/notifier"
	"tailwatch/internal/source"
	"tailwatch/internal/tailscale"
)

type App struct {
	cfg     *config.Config
	logger  *slog.Logger
	resolve func(context.Context, netip.Addr) (*tailscale.Device, error)
	notify  func(context.Context, events.Event) error
}

func New(cfg *config.Config, logger *slog.Logger, output io.Writer, replay bool) *App {
	resolver := tailscale.NewResolver(
		cfg.Tailscale.Binary,
		time.Duration(cfg.Tailscale.LookupTimeoutSeconds)*time.Second,
		time.Duration(cfg.Monitor.IdentityCacheSeconds)*time.Second,
	)
	var webhook string
	if !replay && cfg.Notification.Provider == "webhook" {
		webhook = cfg.Notification.WebhookURL
	}
	return &App{cfg: cfg, logger: logger, resolve: resolver.Resolve, notify: notifier.New(output, webhook).Notify}
}

func (a *App) event(ctx context.Context, session events.Session) (events.Event, error) {
	var device *tailscale.Device
	if session.SourceIP.IsValid() {
		var err error
		device, err = a.resolve(ctx, session.SourceIP)
		if err != nil && ctx.Err() == nil {
			a.logger.Warn("identity lookup failed", "session", session.ID, "ip", session.SourceIP.String(), "error", err)
			device = nil
		}
	}
	if err := ctx.Err(); err != nil {
		return events.Event{}, err
	}
	event := events.Event{Session: session, EventType: "SSH_SESSION_STARTED", Decision: monitor.Classify(a.cfg, session, device)}
	if device != nil && (device.LoginName == "" || device.LoginName == session.TailscaleUserIdentity) {
		event.DeviceHostname = device.Hostname
		event.TailscaleDeviceID = device.ID
	}
	return event, nil
}

// finishPending advances the cursor only after all configured outputs acknowledge the event.
func (a *App) finishPending(ctx context.Context, s *checkpoint, path string) error {
	pending := s.Pending
	if err := a.notify(ctx, pending.Event); err != nil {
		return fmt.Errorf("deliver session %s: %w", pending.Event.ID, err)
	}
	s.Cursor = pending.Cursor
	s.remember(pending.Event.ID)
	s.Pending = nil
	if path != "" {
		return s.save(path)
	}
	return nil
}

type stream struct {
	app       *App
	state     *checkpoint
	statePath string
	lastSave  time.Time
	delivered int
	malformed int
}

func (s *stream) handle(ctx context.Context, record source.Record) error {
	if s.statePath != "" && record.Cursor == "" {
		return errors.New("journal entry has no cursor")
	}
	session, err := record.Session()
	if err != nil {
		s.malformed++
		s.app.logger.Warn("skipping malformed SSH start record", "error", err, "cursor", record.Cursor)
	}
	if session == nil || slices.Contains(s.state.Recent, session.ID) {
		// Limit writes for ordinary daemon chatter. Re-reading these entries is harmless.
		if s.statePath != "" && time.Since(s.lastSave) >= time.Minute {
			s.lastSave = time.Now()
			s.state.Cursor = record.Cursor
			return s.state.save(s.statePath)
		}
		return nil
	}
	event, err := s.app.event(ctx, *session)
	if err != nil {
		return err
	}
	s.state.Pending = &pendingEvent{Cursor: record.Cursor, Event: event}
	if s.statePath != "" {
		if err := s.state.save(s.statePath); err != nil {
			return fmt.Errorf("save undelivered event: %w", err)
		}
	}
	if err := s.app.finishPending(ctx, s.state, s.statePath); err != nil {
		return err
	}
	s.delivered++
	s.lastSave = time.Now()
	s.app.logger.Debug("processed SSH session", "session", event.ID, "classification", event.TrustClassification)
	return nil
}

func (a *App) Replay(ctx context.Context, reader io.Reader) error {
	stream := stream{app: a, state: &checkpoint{}}
	err := source.Read(ctx, reader, func(r source.Record) error { return stream.handle(ctx, r) })
	a.logger.Info("journal replay finished", "sessions", stream.delivered, "malformed", stream.malformed)
	return err
}

func (a *App) Follow(ctx context.Context, statePath string, stderr io.Writer) error {
	if statePath == "" {
		return errors.New("live monitoring requires a state file")
	}
	lock, err := lockJournal(statePath)
	if err != nil {
		return err
	}
	defer lock.Close()
	state, err := loadCheckpoint(statePath)
	if err != nil {
		return fmt.Errorf("open journal state: %w", err)
	}
	if state.Pending != nil {
		if err := a.finishPending(ctx, state, statePath); err != nil {
			return err
		}
	}
	a.logger.Info("following Tailscale SSH journal", "state", statePath, "resuming", state.Cursor != "")
	stream := stream{app: a, state: state, statePath: statePath, lastSave: time.Now()}
	return source.Follow(ctx, state.Since, state.Cursor, stderr, func(r source.Record) error { return stream.handle(ctx, r) })
}
