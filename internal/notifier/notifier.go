package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"tailwatch/internal/events"
)

type Notifier struct {
	output     *json.Encoder
	webhookURL string
	client     *http.Client
}

func New(output io.Writer, webhookURL string) *Notifier {
	return &Notifier{
		output:     json.NewEncoder(output),
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (n *Notifier) Notify(ctx context.Context, event events.Event) error {
	if err := n.output.Encode(event); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	if n.webhookURL == "" || event.Severity != events.SeverityCritical {
		return nil
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return errors.New("invalid webhook request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", event.ID)
	res, err := n.client.Do(req)
	if err != nil {
		// url.Error includes the endpoint, which can contain a secret in its path or query.
		var endpointError *url.Error
		if errors.As(err, &endpointError) {
			err = endpointError.Err
		}
		return fmt.Errorf("webhook POST failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", res.StatusCode)
	}
	return nil
}
