package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"tailwatch/internal/events"
)

const recentLimit = 256

type pendingEvent struct {
	Cursor string       `json:"cursor"`
	Event  events.Event `json:"event"`
}

type checkpoint struct {
	Since   int64         `json:"since"`
	Cursor  string        `json:"cursor,omitempty"`
	Recent  []string      `json:"recent,omitempty"`
	Pending *pendingEvent `json:"pending,omitempty"`
}

func loadCheckpoint(path string) (*checkpoint, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		state := &checkpoint{Since: time.Now().Unix()}
		return state, state.save(path)
	}
	if err != nil {
		return nil, err
	}
	var state checkpoint
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}
	if state.Since <= 0 || len(state.Recent) > recentLimit {
		return nil, errors.New("invalid journal checkpoint")
	}
	if p := state.Pending; p != nil && (p.Cursor == "" || p.Event.ID == "" || p.Event.EventType != "SSH_SESSION_STARTED" ||
		(p.Event.Severity != events.SeverityNormal && p.Event.Severity != events.SeverityCritical)) {
		return nil, errors.New("invalid undelivered event in checkpoint")
	}
	return &state, nil
}

func (s *checkpoint) remember(id string) {
	s.Recent = append(s.Recent, id)
	if len(s.Recent) > recentLimit {
		s.Recent = slices.Delete(s.Recent, 0, 1)
	}
}

func (s *checkpoint) save(path string) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".tailwatch-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(s)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	// The daemon runs on Linux. Persist the rename as well as the file contents.
	if runtime.GOOS == "linux" {
		parent, err := os.Open(dir)
		if err != nil {
			return err
		}
		defer parent.Close()
		return parent.Sync()
	}
	return nil
}
