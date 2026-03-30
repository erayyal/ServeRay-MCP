package httpx

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoJSONRetriesSafeGET(t *testing.T) {
	var attempts atomic.Int32

	client, err := New(Config{
		BaseURL: "https://example.test",
		Timeout: 2 * time.Second,
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			current := attempts.Add(1)
			if current == 1 {
				return newResponse(http.StatusTooManyRequests, `{"message":"retry"}`), nil
			}
			return newResponse(http.StatusOK, `{"value":"ok"}`), nil
		})},
		MinInterval: 0,
		MaxRetries:  1,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	var payload struct {
		Value string `json:"value"`
	}
	if err := client.DoJSON(context.Background(), "/", nil, nil, &payload); err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if payload.Value != "ok" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestDoJSONRespectsContextTimeout(t *testing.T) {
	client, err := New(Config{
		BaseURL: "https://example.test",
		Timeout: 5 * time.Second,
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return newResponse(http.StatusOK, `{"value":"slow"}`), nil
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		})},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var payload map[string]any
	if err := client.DoJSON(ctx, "/", nil, nil, &payload); err == nil {
		t.Fatalf("expected timeout error")
	}
}

func TestNewBlocksPrivateBaseURLByDefault(t *testing.T) {
	if _, err := New(Config{BaseURL: "https://127.0.0.1"}); err == nil {
		t.Fatalf("expected private host to be blocked")
	}
}

func TestNewBlocksInsecureHTTPByDefault(t *testing.T) {
	if _, err := New(Config{BaseURL: "http://example.com"}); err == nil {
		t.Fatalf("expected insecure http to be blocked")
	}
}

func TestNewAllowsPrivateBaseURLWithExplicitOptIn(t *testing.T) {
	if _, err := New(Config{
		BaseURL:           "https://127.0.0.1",
		AllowPrivateHosts: true,
	}); err != nil {
		t.Fatalf("expected explicit private-host opt-in to pass, got %v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func newResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
