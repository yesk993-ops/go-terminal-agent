package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/agent/ai-terminal/internal/core"
)

type webFetchTool struct {
	client *http.Client
}

func NewWebFetchTool() core.Tool {
	return &webFetchTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          10,
				IdleConnTimeout:       30 * time.Second,
				DisableCompression:     false,
				ResponseHeaderTimeout: 15 * time.Second,
			},
		},
	}
}

func (t *webFetchTool) Name() string { return "web_fetch" }

func (t *webFetchTool) Description() string {
	return "Fetch the content of a URL. Useful for reading documentation, articles, or API responses."
}

func (t *webFetchTool) Schema() json.RawMessage {
	return schemaFor(map[string]any{
		"url": map[string]any{
			"type":        "string",
			"description": "The URL to fetch",
		},
		"timeout": map[string]any{
			"type":        "integer",
			"description": "Timeout in seconds (optional, default 30)",
		},
	}, []string{"url"})
}

// isSafeURL validates that a URL uses only http/https schemes and does not
// resolve to a private/internal network address (SSRF prevention).
func isSafeURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := parsed.Scheme
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}

	// Resolve host to check for private/internal IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("cannot resolve host %q: %w", host, err)
	}

	for _, ip := range ips {
		if isPrivateOrUnsafeIP(ip) {
			return fmt.Errorf("URL resolves to a private or internal address (%s), blocked for security", ip.String())
		}
	}

	return nil
}

// isPrivateOrUnsafeIP checks if an IP is in a private, loopback, link-local,
// or other unsafe range that should not be accessible via web_fetch.
func isPrivateOrUnsafeIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip.IsPrivate() {
		return true
	}
	// Block cloud metadata endpoints (both IPv4 and IPv6)
	if ip.To4() != nil {
		// AWS: 169.254.169.254; GCP: 169.254.169.254; Azure: 169.254.169.254
		if ip[0] == 169 && ip[1] == 254 && ip[2] == 169 && ip[3] == 254 {
			return true
		}
	}
	return false
}

func (t *webFetchTool) Execute(ctx context.Context, args json.RawMessage) *core.ToolResult {
	var params struct {
		URL     string `json:"url"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: "invalid arguments: " + err.Error()}
	}

	if err := isSafeURL(params.URL); err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: err.Error()}
	}

	if params.Timeout <= 0 {
		params.Timeout = 30
	}

	client := t.client
	if params.Timeout != 30 {
		client = &http.Client{
			Timeout: time.Duration(params.Timeout) * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
				}).DialContext,
				ResponseHeaderTimeout: 15 * time.Second,
			},
		}
	}

	fetchCtx, cancel := context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, params.URL, nil)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("create request: %v", err)}
	}

	req.Header.Set("User-Agent", "GoAgent/1.0")
	req.Header.Set("Accept", "text/html,text/plain,*/*")

	resp, err := client.Do(req)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("fetch URL: %v", err)}
	}
	defer resp.Body.Close()

	// Limit response body to 1MB to prevent memory exhaustion
	limitedReader := io.LimitReader(resp.Body, 1<<20) // 1MB
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Error: fmt.Sprintf("read body: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return &core.ToolResult{
			Status: core.StatusError,
			Output: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
			Error:  fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
		}
	}

	content := string(body)
	if len(content) > 100000 {
		runes := []rune(content)
		content = string(runes[:100000]) + "\n... (truncated at 100KB)"
	}

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Output: fmt.Sprintf("Content from %s (%d bytes):\n%s", params.URL, len(body), content),
	}
}
