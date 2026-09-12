package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEmailSendRetriesRateLimit(t *testing.T) {
	attempts := 0
	client := &InfraiEmailClient{
		APIKey: "test-key", Endpoint: infraiEmailSend, MaxRetries: 1,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts++
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s", r.Method)
			}
			if r.Header.Get("Idempotency-Key") != "signup-verification:cust-7" {
				t.Fatal("missing idempotency key")
			}
			if attempts == 1 {
				return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"2"}}, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error":{"message":"rate limited"}}`))}, nil
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"data":{"message_id":"msg_7"},"metadata":{}}`))}, nil
		})},
		Sleep: func(_ context.Context, d time.Duration) error {
			if d != 2*time.Second {
				t.Fatalf("delay = %s", d)
			}
			return nil
		},
	}
	id, err := client.Send(context.Background(), Email{To: "buyer@example.com", Subject: "Verify", HTML: "<p>Verify</p>"}, "signup-verification:cust-7")
	if err != nil {
		t.Fatal(err)
	}
	if id != "msg_7" || attempts != 2 {
		t.Fatalf("id = %q, attempts = %d", id, attempts)
	}
}
