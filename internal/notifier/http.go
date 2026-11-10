package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"tailwatch/internal/events"
)

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func postJSON(ctx context.Context, client *http.Client, endpoint string, headers http.Header, payload, response any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid notification endpoint")
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Transport errors can include URLs with bot tokens or webhook credentials.
		return fmt.Errorf("notification HTTP request failed")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("notification endpoint returned HTTP %d", res.StatusCode)
	}
	if response != nil {
		if err := json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(response); err != nil {
			return fmt.Errorf("invalid notification acknowledgement")
		}
	}
	return nil
}

type webhook struct {
	url    string
	client *http.Client
}

func (w webhook) Send(ctx context.Context, e events.Event) error {
	headers := http.Header{"Idempotency-Key": []string{e.ID}}
	return postJSON(ctx, w.client, w.url, headers, envelope(e), nil)
}
