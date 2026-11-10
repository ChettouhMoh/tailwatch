package source

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"tailwatch/internal/events"
)

// OpenSSH emits Accepted once authentication completes, including the final step
// of multi-factor authentication. Subsequent PAM/session records are separate.
var acceptedLogin = regexp.MustCompile(`^Accepted ([^ ]+) for (.+) from ([^ ]+) port ([0-9]+) ssh2([ :].*)?$`)

func (r Record) openSSHSession(msg string) (*events.Session, error) {
	if !strings.HasPrefix(msg, "Accepted ") {
		return nil, nil
	}
	fields := acceptedLogin.FindStringSubmatch(msg)
	if fields == nil {
		return nil, errors.New("unrecognized OpenSSH authentication record")
	}
	ip, err := netip.ParseAddr(fields[3])
	if err != nil {
		return nil, errors.New("invalid peer address in OpenSSH login")
	}
	if _, err := strconv.ParseUint(fields[4], 10, 16); err != nil {
		return nil, errors.New("invalid peer port in OpenSSH login")
	}
	// The trusted journal cursor uniquely identifies a record, even for concurrent
	// logins by the same user. Exports without cursors use the recorded facts.
	key := r.Cursor
	if key == "" {
		key = r.Host + "\x00" + r.Micros + "\x00" + msg
	}
	return &events.Session{
		ID:          fmt.Sprintf("openssh-%x", sha256.Sum256([]byte(key))),
		SSHServer:   events.OpenSSH,
		SSHUsername: fields[2],
		SourceIP:    ip.Unmap(),
		AuthMethod:  fields[1],
	}, nil
}
