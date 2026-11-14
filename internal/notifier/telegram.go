package notifier

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"tailwatch/internal/events"
)

type telegram struct {
	endpoint string
	chatID   string
	client   *http.Client
}

var botToken = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)

func newTelegram(chatID, token string) (*telegram, error) {
	if !botToken.MatchString(token) {
		return nil, errors.New("telegram requires a valid bot token")
	}
	return &telegram{
		endpoint: "https://api.telegram.org/bot" + token + "/sendMessage",
		chatID:   chatID, client: newHTTPClient(),
	}, nil
}

func (t *telegram) Send(ctx context.Context, e events.Event) error {
	payload := map[string]any{
		"chat_id": t.chatID, "text": message(e),
		"link_preview_options": map[string]bool{"is_disabled": true},
	}
	var response struct {
		OK bool `json:"ok"`
	}
	if err := postJSON(ctx, t.client, t.endpoint, nil, payload, &response); err != nil {
		return err
	}
	if !response.OK {
		return errors.New("Telegram did not accept the message")
	}
	return nil
}
