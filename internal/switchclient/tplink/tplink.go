package tplink

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
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
// This is treated as a success signal, but session validity is confirmed with a
// subsequent probe GET to distinguish a real login from a general TCP failure.
func (t *TPLink) Login(ctx context.Context, rawURL, username, password string) error {
	t.baseURL = strings.TrimRight(rawURL, "/")

	// The switch silently ignores POST requests that lack the submit button field.
	_, err := t.http.PostForm(t.baseURL+"/logon.cgi", map[string]string{
		"username": username,
		"password": password,
		"logon":    "Login",
	})
	if err != nil {
		if isConnectionReset(err.Error()) {
			// The TL-SG108E needs up to ~1 s to fully establish the session
			// after the TCP RST before it will serve authenticated pages.
			time.Sleep(1500 * time.Millisecond)
			// Validate the session by fetching a known data page.
			// If credentials were wrong, the switch returns the login form instead.
			if err := t.validateSession(ctx); err != nil {
				return fmt.Errorf("login succeeded (TCP RST) but session validation failed: %w", err)
			}
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
// connection without a response — the TL-SG108E's authentication confirmation signal.
// Deliberately narrow: we do not want to treat general I/O errors as success.
func isConnectionReset(msg string) bool {
	for _, s := range []string{
		"EOF",
		"connection reset by peer",
		"connection reset",
		"broken pipe",
		"use of closed network connection",
		"forcibly closed",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// validateSession confirms the session is authenticated by fetching SystemInfoRpm.htm
// and checking that it returns switch data rather than the login form.
func (t *TPLink) validateSession(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+"/SystemInfoRpm.htm", nil)
	if err != nil {
		return err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return fmt.Errorf("session probe: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read session probe: %w", err)
	}
	// The login page always contains action="/logon.cgi"
	// An authenticated page contains "var info_ds"
	if strings.Contains(string(body), `action="/logon.cgi"`) {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		log.Printf("tplink: session probe returned login form (body[0:200]: %q)", preview)
		return fmt.Errorf("authentication failed: credentials rejected")
	}
	return nil
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

// postAction POSTs form data to a switch page and verifies a successful response.
// The TL-SG108E closes the TCP connection after every POST (same RST behaviour as
// login), so EOF / connection-reset is treated as success just like login.
func (t *TPLink) postAction(ctx context.Context, path string, data map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+path, formBody(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.http.Do(req)
	if err != nil {
		// The TL-SG108E RSTs the connection after every POST, including config
		// writes. There is no HTTP response to confirm the change was applied —
		// RST-as-success is the only signal this firmware provides for any POST.
		if isConnectionReset(err.Error()) {
			return nil
		}
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s: unexpected status %d", path, resp.StatusCode)
	}
	return nil
}

// postMultipart POSTs multipart/form-data to a switch CGI endpoint.
// Used for actions whose form uses enctype=multipart/form-data (e.g. port_setting.cgi).
func (t *TPLink) postMultipart(ctx context.Context, path string, data map[string]string) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range data {
		if err := w.WriteField(k, v); err != nil {
			return fmt.Errorf("build multipart field %s: %w", k, err)
		}
	}
	w.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+path, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := t.http.Do(req)
	if err != nil {
		if isConnectionReset(err.Error()) {
			return nil
		}
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s: unexpected status %d", path, resp.StatusCode)
	}
	return nil
}

// postReboot POSTs a reboot command and accepts a TCP RST as confirmation.
// The switch closes the connection immediately after initiating reboot.
func (t *TPLink) postReboot(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		t.baseURL+"/SystemRebootRpm.htm", formBody(map[string]string{"reboot": "reboot"}))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err = t.http.Do(req)
	if err != nil {
		if isConnectionReset(err.Error()) {
			return nil // expected: switch reboots immediately
		}
		return fmt.Errorf("reboot POST: %w", err)
	}
	return nil
}
