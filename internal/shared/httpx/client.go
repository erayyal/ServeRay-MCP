package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/erayyal/serveray-mcp/internal/shared/limits"
)

type Config struct {
	BaseURL           string
	Timeout           time.Duration
	UserAgent         string
	AuthHeader        string
	MinInterval       time.Duration
	MaxRetries        int
	ResponseSize      int64
	HTTPClient        *http.Client
	AllowHTTP         bool
	AllowPrivateHosts bool
}

type Client struct {
	baseURL      *url.URL
	baseOrigin   string
	httpClient   *http.Client
	userAgent    string
	authHeader   string
	minInterval  time.Duration
	maxRetries   int
	responseSize int64

	mu          sync.Mutex
	lastRequest time.Time
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("remote API returned %d: %s", e.StatusCode, e.Message)
}

func New(cfg Config) (*Client, error) {
	baseURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if err := validateBaseURL(baseURL, cfg.AllowHTTP, cfg.AllowPrivateHosts); err != nil {
		return nil, err
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.MinInterval < 0 {
		cfg.MinInterval = 0
	}
	if cfg.ResponseSize <= 0 {
		cfg.ResponseSize = 1 << 20
	}

	return &Client{
		baseURL:      baseURL,
		baseOrigin:   origin(baseURL),
		httpClient:   pickHTTPClient(cfg, baseURL),
		userAgent:    cfg.UserAgent,
		authHeader:   cfg.AuthHeader,
		minInterval:  cfg.MinInterval,
		maxRetries:   cfg.MaxRetries,
		responseSize: cfg.ResponseSize,
	}, nil
}

func pickHTTPClient(cfg Config, baseURL *url.URL) *http.Client {
	if cfg.HTTPClient != nil {
		client := *cfg.HTTPClient
		if client.Timeout <= 0 {
			client.Timeout = cfg.Timeout
		}
		if client.CheckRedirect == nil {
			client.CheckRedirect = sameOriginRedirectPolicy(origin(baseURL))
		}
		return &client
	}
	return &http.Client{
		Timeout:       cfg.Timeout,
		Transport:     newTransport(cfg.AllowPrivateHosts),
		CheckRedirect: sameOriginRedirectPolicy(origin(baseURL)),
	}
}

func (c *Client) DoJSON(ctx context.Context, path string, query url.Values, headers map[string]string, out any) error {
	var lastErr error
	attempts := limits.Clamp(c.maxRetries+1, 1, 1, 5)

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := c.wait(ctx); err != nil {
			return err
		}

		requestURL := c.baseURL.ResolveReference(&url.URL{Path: path})
		requestURL.RawQuery = query.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		if c.userAgent != "" {
			req.Header.Set("User-Agent", c.userAgent)
		}
		if c.authHeader != "" {
			req.Header.Set("Authorization", c.authHeader)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("perform request: %w", err)
			if attempt == attempts {
				return lastErr
			}
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(res.Body, c.responseSize))
		closeErr := res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close response: %w", closeErr)
		}

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			if out == nil {
				return nil
			}
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			return nil
		}

		message, _ := limits.TruncateString(strings.TrimSpace(string(body)), 512)
		lastErr = &APIError{StatusCode: res.StatusCode, Message: message}
		if res.StatusCode != http.StatusTooManyRequests && res.StatusCode < 500 {
			return lastErr
		}
		if attempt == attempts {
			return lastErr
		}
		time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
	}

	return lastErr
}

func (c *Client) wait(ctx context.Context) error {
	if c.minInterval <= 0 {
		return nil
	}

	c.mu.Lock()
	next := c.lastRequest.Add(c.minInterval)
	now := time.Now()
	sleep := next.Sub(now)
	if sleep <= 0 {
		c.lastRequest = now
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	timer := time.NewTimer(sleep)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}

	c.mu.Lock()
	c.lastRequest = time.Now()
	c.mu.Unlock()
	return nil
}

func validateBaseURL(baseURL *url.URL, allowHTTP, allowPrivateHosts bool) error {
	if baseURL == nil {
		return fmt.Errorf("base URL is required")
	}
	if baseURL.User != nil {
		return fmt.Errorf("base URL must not include embedded credentials")
	}
	switch baseURL.Scheme {
	case "https":
	case "http":
		if !allowHTTP {
			return fmt.Errorf("insecure http base URLs are blocked by default")
		}
	default:
		return fmt.Errorf("base URL scheme must be http or https")
	}
	if baseURL.Hostname() == "" {
		return fmt.Errorf("base URL host is required")
	}
	return validateHost(baseURL.Hostname(), allowPrivateHosts)
}

func validateHost(host string, allowPrivateHosts bool) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return fmt.Errorf("base URL host is required")
	}

	if !allowPrivateHosts {
		switch {
		case host == "localhost":
			return fmt.Errorf("SSRF guard blocked localhost")
		case host == "host.docker.internal":
			return fmt.Errorf("SSRF guard blocked host.docker.internal")
		case strings.HasSuffix(host, ".localhost"):
			return fmt.Errorf("SSRF guard blocked localhost-style host %q", host)
		case strings.HasSuffix(host, ".local"):
			return fmt.Errorf("SSRF guard blocked local host %q", host)
		case strings.HasSuffix(host, ".internal"):
			return fmt.Errorf("SSRF guard blocked internal host %q", host)
		}
	}

	if ip := net.ParseIP(host); ip != nil && !allowPrivateHosts && isDisallowedIP(ip) {
		return fmt.Errorf("SSRF guard blocked private or local host %q", host)
	}

	return nil
}

func newTransport(allowPrivateHosts bool) *http.Transport {
	base := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}

	base.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if err := validateResolvedHost(ctx, host, allowPrivateHosts); err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
	}

	return base
}

func validateResolvedHost(ctx context.Context, host string, allowPrivateHosts bool) error {
	if err := validateHost(host, allowPrivateHosts); err != nil {
		return err
	}
	if allowPrivateHosts {
		return nil
	}

	if ip := net.ParseIP(host); ip != nil {
		if isDisallowedIP(ip) {
			return fmt.Errorf("SSRF guard blocked private or local host %q", host)
		}
		return nil
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve host %q: %w", host, err)
	}
	for _, addr := range addrs {
		if isDisallowedIP(addr.IP) {
			return fmt.Errorf("SSRF guard blocked host %q because it resolved to a private or local address", host)
		}
	}

	return nil
}

func isDisallowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsMulticast() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		switch {
		case ipv4[0] == 0:
			return true
		case ipv4[0] == 100 && ipv4[1]&0b11000000 == 0b01000000:
			return true
		case ipv4[0] == 169 && ipv4[1] == 254:
			return true
		case ipv4[0] == 198 && (ipv4[1] == 18 || ipv4[1] == 19):
			return true
		}
	}
	return false
}

func sameOriginRedirectPolicy(baseOrigin string) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after too many redirects")
		}
		if origin(req.URL) != baseOrigin {
			return fmt.Errorf("redirects outside %s are blocked", baseOrigin)
		}
		return nil
	}
}

func origin(u *url.URL) string {
	if u == nil {
		return ""
	}
	return strings.ToLower(u.Scheme + "://" + u.Host)
}
