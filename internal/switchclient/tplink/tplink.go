package tplink

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/t0mer/SwitchDeck/pkg/httpclient"
)

// TPLink implements switchclient.Client for TP-Link TL-SG series switches.
type TPLink struct {
	baseURL string
	http    *httpclient.Client
}

// New creates a TPLink client. Set insecure=true when the switch uses a
// self-signed cert and the operator accepts the MITM risk on their LAN.
func New(insecure bool) *TPLink {
	return &TPLink{
		http: httpclient.New(httpclient.Options{SkipTLSVerify: insecure}),
	}
}

// Login authenticates to the switch. The TL-SG108E closes the TCP connection
// without sending an HTTP response after receiving valid credentials (TCP RST).
// This is treated as a success signal.
func (t *TPLink) Login(ctx context.Context, rawURL, username, password string) error {
	t.baseURL = strings.TrimRight(rawURL, "/")

	_, err := t.http.PostForm(t.baseURL+"/logon.cgi", map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		if isConnectionReset(err.Error()) {
			time.Sleep(500 * time.Millisecond)
			return nil
		}
		return fmt.Errorf("login POST: %w", err)
	}
	return nil
}

// Logout terminates the switch session.
func (t *TPLink) Logout(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+"/Logout.htm", nil)
	if err != nil {
		return err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	resp.Body.Close()
	return nil
}

// isConnectionReset returns true for errors that indicate the server closed the
// connection without a response (TCP RST — the TL-SG108E's auth confirmation signal).
func isConnectionReset(msg string) bool {
	for _, s := range []string{"EOF", "connection reset", "broken pipe",
		"empty response", "use of closed", "forcibly closed"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// fetchPage GETs a switch page and returns the first script block JS content.
func (t *TPLink) fetchPage(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+path, nil)
	if err != nil {
		return "", err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	js := extractFirstScript(string(body))
	if js == "" {
		return "", fmt.Errorf("no script block in %s", path)
	}
	return js, nil
}

// postAction POSTs form data to a switch page and discards the response.
func (t *TPLink) postAction(ctx context.Context, path string, data map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+path, formBody(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.http.Do(req)
	if err != nil {
		if isConnectionReset(err.Error()) {
			return nil
		}
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}
