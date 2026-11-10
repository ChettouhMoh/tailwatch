package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tailwatch/internal/config"
)

func TestLiveJournalResumeAndLock(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state")
	lock, err := lockJournal(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := lockJournal(statePath); err == nil {
		second.Close()
		t.Fatal("second monitor took the same lock")
	}
	lock.Close()

	fixture, err := filepath.Abs("../../testdata/journal.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAILWATCH_FIXTURE", fixture)
	t.Setenv("TAILWATCH_ARGS", filepath.Join(dir, "args"))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	// First invocation streams the fixture. On resume, block like a real follower.
	command := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$TAILWATCH_ARGS\"\ncase \"$*\" in *--after-cursor=*) exec sleep 60;; esac\ncat \"$TAILWATCH_FIXTURE\"\n"
	if err := os.WriteFile(filepath.Join(dir, "journalctl"), []byte(command), 0700); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	cfg := &config.Config{Tailscale: config.TailscaleConfig{Binary: "/nonexistent-tailscale", LookupTimeoutSeconds: 1}}
	a := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), &output, false)
	var journalErrors bytes.Buffer
	err = a.Follow(context.Background(), statePath, &journalErrors)
	if err == nil || !strings.Contains(err.Error(), "stopped following") {
		t.Fatalf("unexpected journal exit: %v; stderr: %s", err, journalErrors.String())
	}
	if strings.Count(output.String(), "\n") != 4 {
		t.Fatalf("wrong output: %s", output.String())
	}
	args, err := os.ReadFile(os.Getenv("TAILWATCH_ARGS"))
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{"--since=@", "--follow", "--no-tail", "_SYSTEMD_UNIT=tailscaled.service", "_COMM=tailscaled\n+\n_UID=0\n_COMM=sshd\n_COMM=sshd-session\n_COMM=sshd-auth"} {
		if !strings.Contains(string(args), arg) {
			t.Fatalf("missing %s in %s", arg, args)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Follow(ctx, statePath, io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected stop: %v", err)
	}
	args, err = os.ReadFile(os.Getenv("TAILWATCH_ARGS"))
	if err != nil || !strings.Contains(string(args), "--after-cursor=c7") {
		t.Fatalf("resume args=%s error=%v", args, err)
	}
}
