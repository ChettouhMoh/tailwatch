package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"tailwatch/internal/config"
	"tailwatch/internal/events"
	"tailwatch/internal/source"
	"tailwatch/internal/tailscale"
)

func TestFailedDeliverySurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	state, err := loadCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	msg, _ := json.Marshal(`ssh-session(sess-original): handling new SSH connection from admin (100.100.1.2) to ssh-user "ops"`)
	record := source.Record{Unit: "tailscaled.service", Command: "tailscaled", Host: "server", Cursor: "c1", Micros: "1768468800000000", Message: msg}
	ctx := context.Background()
	a := New(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), io.Discard, false)
	lookups := 0
	a.resolve = func(context.Context, netip.Addr) (*tailscale.Device, error) {
		lookups++
		return &tailscale.Device{ID: "n-1", Hostname: "laptop", LoginName: "admin"}, nil
	}
	offline := errors.New("offline")
	a.notify = func(context.Context, events.Event) error { return offline }
	reader := stream{app: a, state: state, statePath: path}
	if err := reader.handle(ctx, record); !errors.Is(err, offline) {
		t.Fatalf("unexpected delivery result: %v", err)
	}
	recovered, err := loadCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Cursor != "" || recovered.Pending == nil || recovered.Pending.Cursor != "c1" || recovered.Pending.Event.TailscaleDeviceID != "n-1" {
		t.Fatalf("lost undelivered alert: %+v", recovered)
	}
	var sent []events.Event
	a.notify = func(_ context.Context, event events.Event) error { sent = append(sent, event); return nil }
	if err := a.finishPending(ctx, recovered, path); err != nil {
		t.Fatal(err)
	}

	recovered, err = loadCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Cursor != "c1" || recovered.Pending != nil {
		t.Fatalf("bad recovered state: %+v", recovered)
	}
	reader = stream{app: a, state: recovered, statePath: path}
	record.Cursor = "c2"
	if err := reader.handle(ctx, record); err != nil {
		t.Fatal(err)
	}
	if lookups != 1 || len(sent) != 1 || sent[0].ID != "sess-original" || sent[0].TrustClassification != events.UnrecognizedTailnet {
		t.Fatalf("lookups=%d sent=%+v", lookups, sent)
	}
}

func TestCorruptCheckpointDoesNotResetCursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	for _, data := range []string{"{", "{}", `{"since":1,"pending":{"cursor":"c1"}}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadCheckpoint(path); err == nil {
			t.Fatalf("unexpectedly reset %s", data)
		}
	}
}

func TestStandaloneGrantProducesUnknownAlert(t *testing.T) {
	cfg := &config.Config{TrustedUsers: []string{"admin"}}
	a := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), io.Discard, true)
	a.resolve = func(context.Context, netip.Addr) (*tailscale.Device, error) {
		t.Fatal("grant without IP triggered identity lookup")
		return nil, nil
	}
	msg, _ := json.Marshal(`ssh-session(sess-fallback): access granted to admin as ssh-user "ops"`)
	record := source.Record{Unit: "tailscaled.service", Command: "tailscaled", Host: "server", Micros: "1768468800000000", Message: msg}
	var sent []events.Event
	a.notify = func(_ context.Context, event events.Event) error { sent = append(sent, event); return nil }
	stream := stream{app: a, state: &checkpoint{}}
	for i := 0; i < 2; i++ {
		if err := stream.handle(context.Background(), record); err != nil {
			t.Fatal(err)
		}
	}
	if len(sent) != 1 || sent[0].TrustClassification != events.Unknown || sent[0].SourceIP.IsValid() {
		t.Fatalf("unexpected fallback events: %+v", sent)
	}
}

func TestOpenSSHPeerEnrichment(t *testing.T) {
	cfg := &config.Config{TrustedUsers: []string{"alice@example.com"}}
	a := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), io.Discard, true)
	calls := 0
	a.resolve = func(context.Context, netip.Addr) (*tailscale.Device, error) {
		calls++
		return &tailscale.Device{ID: "n-1", Hostname: "laptop", LoginName: "alice@example.com"}, nil
	}
	session := events.Session{SSHServer: events.OpenSSH, SSHUsername: "root", SourceIP: netip.MustParseAddr("100.100.1.2")}
	peer, err := a.event(context.Background(), session)
	if err != nil || peer.PeerLogin != "alice@example.com" || peer.SSHUsername != "root" || peer.TailscaleUserIdentity != "" || peer.TrustClassification != events.Trusted {
		t.Fatalf("wrong peer attribution: %+v %v", peer, err)
	}
	for _, addr := range []string{"203.0.113.5", "192.168.1.1"} {
		session.SourceIP = netip.MustParseAddr(addr)
		plain, err := a.event(context.Background(), session)
		if err != nil || plain.PeerLogin != "" || plain.TailscaleDeviceID != "" || calls != 1 {
			t.Fatalf("unexpected public/LAN attribution: %+v %v", plain, err)
		}
	}
}
