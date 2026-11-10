package source

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tailwatch/internal/events"
)

// journalctl encodes timestamps as decimal strings of microseconds since the Unix epoch.
type Record struct {
	Cursor  string          `json:"__CURSOR"`
	Micros  string          `json:"__REALTIME_TIMESTAMP"`
	Host    string          `json:"_HOSTNAME"`
	Unit    string          `json:"_SYSTEMD_UNIT"`
	Command string          `json:"_COMM"`
	UID     string          `json:"_UID"`
	Message json.RawMessage `json:"MESSAGE"`
}

var (
	sessionPrefix = regexp.MustCompile(`^ssh-session\((sess-[A-Za-z0-9-]+)\): (.*)$`)
	startDetails  = regexp.MustCompile(`^handling new SSH connection from (.*?) \(([^() ]+)\) to ssh-user (".*")$`)
	grantDetails  = regexp.MustCompile(`^access granted to (.*?) as ssh-user (".*")$`)
)

func (r Record) Session() (*events.Session, error) {
	var msg string
	if len(r.Message) == 0 || r.Message[0] != '"' {
		return nil, nil
	}
	if err := json.Unmarshal(r.Message, &msg); err != nil {
		return nil, err
	}
	var session *events.Session
	var err error
	switch {
	case r.Unit == "tailscaled.service" && r.Command == "tailscaled":
		session, err = tailscaleSession(msg)
	case r.UID == "0" && (r.Command == "sshd" || r.Command == "sshd-session" || r.Command == "sshd-auth"):
		session, err = r.openSSHSession(msg)
	}
	if err != nil || session == nil {
		return nil, err
	}
	micros, err := strconv.ParseInt(r.Micros, 10, 64)
	if err != nil || micros <= 0 || r.Host == "" {
		return nil, errors.New("missing journal host or valid timestamp")
	}
	session.Timestamp = time.UnixMicro(micros).UTC()
	session.Server = r.Host
	return session, nil
}

// The detailed start and adjacent access grant describe the same Tailscale session.
func tailscaleSession(msg string) (*events.Session, error) {
	msg = strings.TrimPrefix(msg, "ssh: ")
	header := sessionPrefix.FindStringSubmatch(msg)
	if header == nil {
		return nil, nil
	}
	var login, account string
	var ip netip.Addr
	detail := header[2]
	switch {
	case strings.HasPrefix(detail, "handling new SSH connection"):
		fields := startDetails.FindStringSubmatch(detail)
		if fields == nil {
			return nil, errors.New("unrecognized Tailscale SSH start message format")
		}
		addr, err := netip.ParseAddr(fields[2])
		if err != nil || addr.Zone() != "" {
			return nil, errors.New("invalid source IP in SSH start message")
		}
		ip, login, account = addr.Unmap(), fields[1], fields[3]
	case strings.HasPrefix(detail, "access granted"):
		fields := grantDetails.FindStringSubmatch(detail)
		if fields == nil {
			return nil, errors.New("unrecognized Tailscale SSH grant message format")
		}
		login, account = fields[1], fields[2]
	default:
		return nil, nil
	}
	user, err := strconv.Unquote(account)
	if err != nil || user == "" {
		return nil, errors.New("invalid SSH account in start message")
	}
	return &events.Session{
		ID:                    header[1],
		SSHServer:             events.TailscaleSSH,
		SourceIP:              ip,
		SSHUsername:           user,
		TailscaleUserIdentity: login,
	}, nil
}

func Read(ctx context.Context, reader io.Reader, handle func(Record) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for line := 1; scanner.Scan(); line++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return fmt.Errorf("journal JSON at line %d: %w", line, err)
		}
		if err := handle(record); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read journal: %w", err)
	}
	return ctx.Err()
}

func Follow(ctx context.Context, since int64, cursor string, stderr io.Writer, handle func(Record) error) error {
	// --no-tail overrides --follow's implicit last-ten-records limit during catch-up.
	start := "--since=@" + strconv.FormatInt(since, 10)
	if cursor != "" {
		start = "--after-cursor=" + cursor
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(child, "journalctl", "--follow", "--no-tail", "--no-pager", "--output=json",
		"--output-fields=MESSAGE,_HOSTNAME,_SYSTEMD_UNIT,_COMM,_UID", start,
		"_SYSTEMD_UNIT=tailscaled.service", "_COMM=tailscaled", "+",
		"_UID=0", "_COMM=sshd", "_COMM=sshd-session", "_COMM=sshd-auth")
	cmd.Stderr = stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start journalctl: %w", err)
	}
	readErr := Read(ctx, out, handle)
	if readErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if readErr != nil {
		return readErr
	}
	if waitErr != nil {
		return fmt.Errorf("journalctl stopped: %w", waitErr)
	}
	return errors.New("journalctl stopped following the journal")
}
