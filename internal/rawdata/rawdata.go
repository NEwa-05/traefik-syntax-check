// Package rawdata retrieves and decodes the runtime configuration exposed by
// the Traefik v2 API on the /api/rawdata endpoint.
package rawdata

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const rawDataPath = "/api/rawdata"

// Router holds the router fields relevant for the rule validation.
// Every other field returned by the API is ignored.
type Router struct {
	EntryPoints []string `json:"entryPoints,omitempty"`
	Service     string   `json:"service,omitempty"`
	Rule        string   `json:"rule,omitempty"`
	RuleSyntax  string   `json:"ruleSyntax,omitempty"`
	Priority    int      `json:"priority,omitempty"`
	Status      string   `json:"status,omitempty"`
}

// RawData is the subset of the /api/rawdata payload holding routers.
// UDP routers are not decoded since they have no rule.
type RawData struct {
	Routers    map[string]Router `json:"routers,omitempty"`
	TCPRouters map[string]Router `json:"tcpRouters,omitempty"`
}

// FetchOptions configures the retrieval of the raw data from the API.
type FetchOptions struct {
	// Headers are added to the request (e.g. Authorization).
	Headers http.Header
	// BasicAuth is a "user:password" pair, used when not empty.
	BasicAuth string
	// Insecure skips the TLS certificate verification.
	Insecure bool
	Timeout  time.Duration
}

// URL builds the /api/rawdata URL from a Traefik base URL.
// The base URL can point to the API root, the dashboard, or the endpoint itself.
func URL(base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parsing URL %q: %w", base, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid URL %q: scheme and host are required", base)
	}

	p := strings.TrimSuffix(u.Path, "/")
	if !strings.HasSuffix(p, rawDataPath) {
		p = strings.TrimSuffix(p, "/dashboard")
		p = strings.TrimSuffix(p, "/api")
		p += rawDataPath
	}
	u.Path = p
	u.RawPath = ""

	return u.String(), nil
}

// Fetch retrieves the raw data from the Traefik API at the given base URL.
// It returns the decoded data along with the raw body.
func Fetch(ctx context.Context, base string, opts FetchOptions) (*RawData, []byte, error) {
	endpoint, err := URL(base)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}
	for k, values := range opts.Headers {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}
	if opts.BasicAuth != "" {
		user, pass, _ := strings.Cut(opts.BasicAuth, ":")
		req.SetBasicAuth(user, pass)
	}
	req.Header.Set("Accept", "application/json")

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if opts.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // Explicitly requested by the user.
	}
	client := &http.Client{Transport: transport, Timeout: opts.Timeout}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading response from %s: %w", endpoint, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("fetching %s: unexpected status %s", endpoint, resp.Status)
	}

	data, err := Decode(body)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding response from %s: %w", endpoint, err)
	}

	return data, body, nil
}

// ReadFile reads raw data previously saved from the /api/rawdata endpoint.
func ReadFile(path string) (*RawData, error) {
	body, err := os.ReadFile(path) //nolint:gosec // The path is provided by the user on purpose.
	if err != nil {
		return nil, err
	}

	data, err := Decode(body)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}

	return data, nil
}

// Decode decodes a /api/rawdata JSON payload.
func Decode(body []byte) (*RawData, error) {
	var data RawData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	return &data, nil
}
