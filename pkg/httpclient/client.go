package httpclient

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

// Options configures the HTTP client.
type Options struct {
	SkipTLSVerify bool
	Timeout       time.Duration
}

// Client wraps net/http.Client with a cookie jar and optional TLS skip.
type Client struct {
	http *http.Client
}

// New creates a Client with a cookie jar and the given options.
func New(opts Options) *Client {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.SkipTLSVerify, //nolint:gosec // operator-controlled flag
		},
	}
	return &Client{
		http: &http.Client{
			Jar:       jar,
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// Do executes an HTTP request.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.http.Do(req)
}

// Get performs a GET request.
func (c *Client) Get(u string) (*http.Response, error) {
	return c.http.Get(u)
}

// PostForm performs a POST with form-encoded body.
func (c *Client) PostForm(u string, data map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, u, formBody(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.http.Do(req)
}

func formBody(data map[string]string) *strings.Reader {
	vals := url.Values{}
	for k, v := range data {
		vals.Set(k, v)
	}
	return strings.NewReader(vals.Encode())
}
