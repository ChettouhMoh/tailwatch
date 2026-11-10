package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"tailwatch/internal/config"
	"tailwatch/internal/events"
)

// Sender acknowledges an event when the channel has accepted it. Returning an
// error leaves the original event pending for retry.
type Sender interface {
	Send(context.Context, events.Event) error
}

type Output struct {
	local  *json.Encoder
	remote Sender
}

func NewOutput(w io.Writer, remote Sender) *Output {
	return &Output{local: json.NewEncoder(w), remote: remote}
}

func (o *Output) Send(ctx context.Context, event events.Event) error {
	if err := o.local.Encode(envelope(event)); err != nil {
		return fmt.Errorf("write login event: %w", err)
	}
	if o.remote == nil {
		return nil
	}
	return o.remote.Send(ctx, event)
}

func Remote(cfg config.NotificationConfig) (Sender, error) {
	switch cfg.Provider {
	case "", "stdout":
		return nil, nil
	case "webhook":
		return webhook{url: cfg.WebhookURL, client: newHTTPClient()}, nil
	case "telegram":
		return newTelegram(cfg.Telegram.ChatID)
	case "whatsapp":
		return newWhatsApp(cfg.WhatsApp)
	default:
		return nil, fmt.Errorf("unknown notification provider %q", cfg.Provider)
	}
}
