package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"tailwatch/internal/config"
	"tailwatch/internal/events"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMessagingProviders(t *testing.T) {
	event := events.Event{Session: events.Session{ID: "sess-1", SSHServer: events.OpenSSH, Server: "vps", SSHUsername: "deploy"}, Decision: events.Decision{TrustClassification: events.Trusted, Severity: events.SeverityNormal}}
	t.Setenv("TAILWATCH_TELEGRAM_BOT_TOKEN", "123:local_test")
	t.Setenv("TAILWATCH_WHATSAPP_ACCESS_TOKEN", "test-token")
	telegramConfig := config.NotificationConfig{Provider: "telegram"}
	telegramConfig.Telegram.ChatID = "-123"
	waConfig := config.NotificationConfig{Provider: "whatsapp", WhatsApp: config.WhatsAppConfig{
		PhoneNumberID: "1234", Recipient: "15550100000", APIVersion: "v23.0", Template: "vps_login", Language: "en_US",
	}}
	for _, tc := range []struct {
		cfg                     config.NotificationConfig
		url, authorization, ack string
		payload                 map[string]any
	}{
		{telegramConfig, "https://api.telegram.org/bot123:local_test/sendMessage", "", "{\"ok\":true}", map[string]any{
			"chat_id": "-123", "text": message(event), "link_preview_options": map[string]any{"is_disabled": true},
		}},
		{waConfig, "https://graph.facebook.com/v23.0/1234/messages", "Bearer test-token", `{"messages":[{"id":"wamid.test"}]}`, map[string]any{
			"messaging_product": "whatsapp", "type": "template", "to": "15550100000", "template": map[string]any{
				"name": "vps_login", "language": map[string]any{"code": "en_US"}, "components": []any{map[string]any{
					"type": "body", "parameters": []any{map[string]any{"type": "text", "text": message(event)}},
				}},
			},
		}},
	} {
		t.Run(tc.cfg.Provider, func(t *testing.T) {
			sender, err := Remote(tc.cfg)
			if err != nil {
				t.Fatal(err)
			}
			ack := tc.ack
			status := http.StatusOK
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != "POST" || req.URL.String() != tc.url || req.Header.Get("Authorization") != tc.authorization || req.Header.Get("Content-Type") != "application/json" {
					t.Fatal("incorrect endpoint, headers or method")
				}
				var got map[string]any
				if err := json.NewDecoder(req.Body).Decode(&got); err != nil || !reflect.DeepEqual(got, tc.payload) {
					t.Fatalf("payload=%+v err=%v", got, err)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(ack)), Header: make(http.Header)}, nil
			})
			switch s := sender.(type) {
			case *telegram:
				s.client.Transport = transport
			case *whatsApp:
				s.client.Transport = transport
			default:
				t.Fatalf("unexpected sender %T", s)
			}
			if err := sender.Send(context.Background(), event); err != nil {
				t.Fatal(err)
			}
			for _, bad := range []string{"{}", `{"ok":false,"messages":[]}`, "broken"} {
				ack = bad
				if err := sender.Send(context.Background(), event); err == nil {
					t.Fatal("accepted invalid acknowledgement")
				}
			}
			status = http.StatusTooManyRequests
			if err := sender.Send(context.Background(), event); err == nil || !strings.Contains(err.Error(), "429") {
				t.Fatalf("rate limit error: %v", err)
			}
		})
	}
}

func TestMissingChatCredentials(t *testing.T) {
	t.Setenv("TAILWATCH_TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TAILWATCH_WHATSAPP_ACCESS_TOKEN", "")
	for _, name := range []string{"telegram", "whatsapp"} {
		if _, err := Remote(config.NotificationConfig{Provider: name}); err == nil {
			t.Fatalf("%s accepted missing credentials", name)
		}
	}
}
