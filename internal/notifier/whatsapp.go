package notifier

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode"

	"tailwatch/internal/config"
	"tailwatch/internal/events"
)

type whatsApp struct {
	config config.WhatsAppConfig
	token  string
	client *http.Client
}

func newWhatsApp(cfg config.WhatsAppConfig, token string) (*whatsApp, error) {
	if token == "" || strings.IndexFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) != -1 {
		return nil, errors.New("whatsapp requires a valid access token")
	}
	return &whatsApp{config: cfg, token: token, client: newHTTPClient()}, nil
}

func (w *whatsApp) Send(ctx context.Context, e events.Event) error {
	endpoint := "https://graph.facebook.com/" + w.config.APIVersion + "/" + w.config.PhoneNumberID + "/messages"
	body := map[string]any{
		"type":       "body",
		"parameters": []map[string]string{{"type": "text", "text": message(e)}},
	}
	payload := map[string]any{
		"messaging_product": "whatsapp", "type": "template", "to": w.config.Recipient,
		"template": map[string]any{
			"name": w.config.Template, "language": map[string]string{"code": w.config.Language},
			"components": []map[string]any{body},
		},
	}
	headers := http.Header{"Authorization": []string{"Bearer " + w.token}}
	var response struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := postJSON(ctx, w.client, endpoint, headers, payload, &response); err != nil {
		return err
	}
	if len(response.Messages) == 0 || response.Messages[0].ID == "" {
		return errors.New("WhatsApp did not return a message ID")
	}
	return nil
}
