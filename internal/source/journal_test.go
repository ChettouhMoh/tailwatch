package source

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSessionRecord(t *testing.T) {
	const message = `ssh-session(sess-20260115T092000-aabbccddee): handling new SSH connection from alice@example.com (100.100.12.5) to ssh-user "deploy"`
	for _, tc := range []struct {
		name, unit, comm, msg string
		want, invalid         bool
	}{
		{"start", "tailscaled.service", "tailscaled", message, true, false},
		{"prefix and IPv6", "tailscaled.service", "tailscaled", "ssh: " + strings.Replace(message, "100.100.12.5", "fd7a:115c:a1e0::1", 1), true, false},
		{"grant fallback", "tailscaled.service", "tailscaled", `ssh-session(sess-20260115T092000-aabbccddee): access granted to alice@example.com as ssh-user "deploy"`, true, false},
		{"failed authentication", "tailscaled.service", "tailscaled", "ssh-conn-1: access denied", false, false},
		{"another service", "sshd.service", "tailscaled", message, false, false},
		{"another process", "tailscaled.service", "logger", message, false, false},
		{"embedded message", "tailscaled.service", "tailscaled", "warning: " + message, false, false},
		{"bad address", "tailscaled.service", "tailscaled", strings.Replace(message, "100.100.12.5", "bad", 1), false, true},
		{"changed format", "tailscaled.service", "tailscaled", strings.Replace(message, "to ssh-user", "as account", 1), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(tc.msg)
			record := Record{Unit: tc.unit, Command: tc.comm, Message: data, Micros: "1768468800123456", Host: "vps"}
			session, err := record.Session()
			if (err != nil) != tc.invalid || (session != nil) != tc.want {
				t.Fatalf("session=%+v error=%v", session, err)
			}
			if session != nil && (session.ID != "sess-20260115T092000-aabbccddee" || session.TailscaleUserIdentity != "alice@example.com" ||
				session.SSHUsername != "deploy" || session.Server != "vps" || session.Timestamp.UnixMicro() != 1768468800123456) {
				t.Fatalf("incorrect session: %+v", session)
			}
		})
	}
}

func TestReadRejectsBrokenExport(t *testing.T) {
	err := Read(context.Background(), strings.NewReader("{}\nnot-json"), func(Record) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("expected line number; got %v", err)
	}
}
