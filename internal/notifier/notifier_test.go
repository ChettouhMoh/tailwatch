package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tailwatch/internal/events"
)

func TestWebhookSendsOnlyCriticalEvents(t *testing.T) {
	received := make(chan events.Event, 2)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event events.Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Idempotency-Key") != event.ID {
			t.Error("incorrect webhook request")
		}
		received <- event
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	var stdout bytes.Buffer
	n := New(&stdout, server.URL)
	n.client.Transport = server.Client().Transport
	e := events.Event{Session: events.Session{ID: "sess-1"}, Decision: events.Decision{Severity: events.SeverityNormal}}
	if err := n.Notify(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	e.ID = "sess-2"
	e.Severity = events.SeverityCritical
	if err := n.Notify(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || (<-received).ID != "sess-2" || strings.Count(stdout.String(), "\n") != 2 {
		t.Fatalf("wrong delivery count; stdout=%s", stdout.String())
	}
}

func TestWebhookFailureAndRedirect(t *testing.T) {
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusTemporaryRedirect} {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/elsewhere" {
				t.Error("followed redirect")
			}
			w.Header().Set("Location", "/elsewhere")
			w.WriteHeader(status)
		}))
		n := New(io.Discard, server.URL+"/private-token")
		n.client.Transport = server.Client().Transport
		event := events.Event{Decision: events.Decision{Severity: events.SeverityCritical}}
		err := n.Notify(context.Background(), event)
		server.Close()
		if err == nil || strings.Contains(err.Error(), "private-token") {
			t.Fatalf("unexpected delivery error: %v", err)
		}
		err = n.Notify(context.Background(), event)
		if err == nil || strings.Contains(err.Error(), "private-token") {
			t.Fatalf("endpoint leaked or missing error: %v", err)
		}
	}
}
