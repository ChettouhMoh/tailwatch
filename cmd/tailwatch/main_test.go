package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	for _, want := range []events.TrustClassification{events.Trusted, events.Unknown} {
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
