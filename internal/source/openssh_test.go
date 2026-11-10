package source

import (
	"encoding/json"
	"testing"

	"tailwatch/internal/events"
)

func TestOpenSSHSuccessRecords(t *testing.T) {
	const accepted = "Accepted keyboard-interactive/pam for ACME user from 2001:db8::3 port 58021 ssh2"
	for _, tc := range []struct {
		comm, uid, msg string
		want           bool
	}{
		{"sshd", "0", accepted, true}, {"sshd-session", "0", accepted, true}, {"sshd-auth", "0", accepted, true},
		{"logger", "0", accepted, false}, {"sshd", "1001", accepted, false},
		{"sshd", "0", "Failed password for root from 198.51.100.2 port 10000 ssh2", false},
		{"sshd", "0", "Partial publickey for root from 198.51.100.2 port 10000 ssh2", false},
		{"sshd-session", "0", "pam_unix(sshd:session): session opened for user root", false},
	} {
		msg, _ := json.Marshal(tc.msg)
		rec := Record{Command: tc.comm, UID: tc.uid, Host: "vps", Micros: "1768468805000000", Cursor: "c7", Message: msg}
		session, err := rec.Session()
		if err != nil || (session != nil) != tc.want {
			t.Fatalf("%+v: session=%v err=%v", tc, session, err)
		}
		if !tc.want {
			continue
		}
		if session.SSHServer != events.OpenSSH || session.AuthMethod != "keyboard-interactive/pam" || session.SSHUsername != "ACME user" || session.SourceIP.String() != "2001:db8::3" {
			t.Fatalf("wrong event: %+v", session)
		}
		again, _ := rec.Session()
		rec.Cursor = "different"
		other, _ := rec.Session()
		if session.ID != again.ID || session.ID == other.ID {
			t.Fatal("unstable or colliding journal event IDs")
		}
	}
}

func TestOpenSSHGrantWithoutIPInBinaryJournalField(t *testing.T) {
	message := "Accepted publickey for ops from UNKNOWN port 65535 ssh2"
	octets := make([]int, len(message))
	for i := range octets {
		octets[i] = int(message[i])
	}
	raw, err := json.Marshal(octets)
	if err != nil {
		t.Fatal(err)
	}
	record := Record{Command: "sshd", UID: "0", Host: "vps", Micros: "1768468805000000", Message: raw}
	session, err := record.Session()
	if err != nil || session == nil || session.SSHUsername != "ops" || session.AuthMethod != "publickey" || session.SourceIP.IsValid() {
		t.Fatalf("lost confirmed authentication: session=%+v err=%v", session, err)
	}
}
