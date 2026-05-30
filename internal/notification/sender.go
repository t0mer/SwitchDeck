package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containrrr/shoutrrr"
)

// Sender sends a notification message to a single channel.
type Sender interface {
	Send(ctx context.Context, message string) error
}

// NewSender constructs a Sender for the given channel's provider.
func NewSender(ch *Channel) (Sender, error) {
	switch ch.Provider {
	case ProviderShoutrrr:
		var cfg ShoutrrrConfig
		if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
			return nil, fmt.Errorf("shoutrrr config: %w", err)
		}
		return &shoutrrrSender{url: strings.TrimSpace(cfg.URL)}, nil
	case ProviderGreenAPI:
		var cfg GreenAPIConfig
		if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
			return nil, fmt.Errorf("greenapi config: %w", err)
		}
		apiURL := strings.TrimSpace(cfg.APIURL)
		if apiURL == "" {
			apiURL = "https://api.green-api.com"
		}
		return &greenAPISender{
			instanceID: strings.TrimSpace(cfg.InstanceID),
			token:      strings.TrimSpace(cfg.Token),
			recipient:  strings.TrimSpace(cfg.Recipient),
			apiURL:     apiURL,
		}, nil
	case ProviderWhatsApp:
		var cfg WhatsAppConfig
		if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
			return nil, fmt.Errorf("whatsapp config: %w", err)
		}
		return &whatsappSender{
			baseURL:   strings.TrimSpace(cfg.BaseURL),
			recipient: strings.TrimSpace(cfg.Recipient),
			username:  cfg.Username,
			password:  cfg.Password,
		}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", ch.Provider)
	}
}

type shoutrrrSender struct{ url string }

func (s *shoutrrrSender) Send(_ context.Context, message string) error {
	return shoutrrr.Send(s.url, message)
}

type greenAPISender struct {
	instanceID, token, recipient, apiURL string
}

func (s *greenAPISender) Send(ctx context.Context, message string) error {
	chatID := s.recipient
	if !strings.Contains(chatID, "@") {
		chatID += "@c.us"
	}
	endpoint := fmt.Sprintf("%s/waInstance%s/sendMessage/%s", s.apiURL, s.instanceID, s.token)
	body, _ := json.Marshal(map[string]string{"chatId": chatID, "message": message})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("greenapi: unexpected status %d", resp.StatusCode)
	}
	return nil
}

type whatsappSender struct {
	baseURL, recipient, username, password string
}

func (s *whatsappSender) Send(ctx context.Context, message string) error {
	body, _ := json.Marshal(map[string]string{"phone": s.recipient, "message": message})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/send/message", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("whatsapp: unexpected status %d", resp.StatusCode)
	}
	return nil
}
