package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tailwatch/internal/events"
)

func TestCLIReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	state := filepath.Join(dir, "state")
	t.Setenv("TAILWATCH_WEBHOOK_URL", "")
	data := `{"tailscale":{"binary":"tailwatch-missing-binary"},"trusted_ips":["100.100.12.5"],"notification":{"provider":"webhook","webhook_url":"https://127.0.0.1:1/unreachable"}}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	var output, logs bytes.Buffer
	if err := run(context.Background(), []string{"-config", path, "-state", state, "-replay", "../../testdata/journal.jsonl"}, &output, &logs); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	for _, want := range []events.TrustClassification{events.Trusted, events.Unknown, events.External, events.Unknown} {
		var event events.Event
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		if event.TrustClassification != want || event.Server != "vps-demo" || event.ID == "" {
			t.Fatalf("unexpected event: %+v", event)
		}
	}
	if err := decoder.Decode(new(events.Event)); !errors.Is(err, io.EOF) {
		t.Fatal("unexpected extra output")
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("replay wrote live state: %v", err)
	}
}

func TestCheckPrivateChannel(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "config.json")
	channel := filepath.Join(dir, "channel.json")
	state := filepath.Join(dir, "journal.json")
	if err := os.WriteFile(base, []byte(`{"notification":{"provider":"webhook"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAILWATCH_WEBHOOK_URL", "https://legacy.invalid/test-secret")
	t.Setenv("TAILWATCH_TELEGRAM_BOT_TOKEN", "123:old")
	for _, tc := range []struct {
		channel, provider string
	}{
		{"", "webhook"},
		{`{"provider":"telegram","telegram":{"chat_id":"123"},"credential":"123:test-secret"}`, "telegram"},
		{`{"provider":"telegram","telegram":{"chat_id":"123"},"credential":""}`, ""},
		{`{"provider":"webhook","credential":"http://test.invalid/test-secret"}`, ""},
		{`{"provider":"stdout","test-secret":true}`, ""},
		{`{"provider":"stdout"} {}`, ""},
		{`{"provider":"stdout"}`, "stdout"},
	} {
		if err := os.WriteFile(channel, []byte(tc.channel), 0600); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		err := run(context.Background(), []string{"-config", base, "-channel", channel, "-check", "-state", state}, &output, &output)
		if (err == nil) != (tc.provider != "") || !strings.Contains(output.String(), tc.provider) {
			t.Fatalf("provider %q, output %q, err %v", tc.provider, output.String(), err)
		}
		if strings.Contains(output.String(), "test-secret") || (err != nil && strings.Contains(err.Error(), "test-secret")) {
			t.Fatal("leaked credential in diagnostic output")
		}
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("config check touched the live journal state")
	}
}

func TestReplayWithoutChatCredentials(t *testing.T) {
	t.Setenv("TAILWATCH_TELEGRAM_BOT_TOKEN", "")
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := `{"tailscale":{"binary":"missing-tailscale"},"notification":{"provider":"telegram","telegram":{"chat_id":"123"}}}`
	if err := os.WriteFile(path, []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"-config", path, "-replay", "../../testdata/journal.jsonl"}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	if bytes.Count(output.Bytes(), []byte("\n")) != 4 {
		t.Fatal("missing replay events")
	}
}
