package notification

import "time"

const (
	ProviderShoutrrr = "shoutrrr"
	ProviderGreenAPI = "greenapi"
	ProviderWhatsApp = "whatsapp_web"
)

// Channel is a configured notification destination.
type Channel struct {
	ID            string
	Name          string
	Provider      string
	Config        string // decrypted JSON, plaintext in memory only
	Enabled       bool
	NotifyOffline bool
	NotifyOnline  bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ShoutrrrConfig holds the single URL for a Shoutrrr channel.
type ShoutrrrConfig struct {
	URL string `json:"url"`
}

// GreenAPIConfig holds credentials for the GreenAPI WhatsApp provider.
type GreenAPIConfig struct {
	InstanceID string `json:"instance_id"`
	Token      string `json:"token"`
	Recipient  string `json:"recipient"`
	APIURL     string `json:"api_url,omitempty"` // defaults to https://api.green-api.com
}

// WhatsAppConfig holds connection details for a self-hosted WhatsApp Web instance.
type WhatsAppConfig struct {
	BaseURL   string `json:"base_url"`
	Recipient string `json:"recipient"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
}
