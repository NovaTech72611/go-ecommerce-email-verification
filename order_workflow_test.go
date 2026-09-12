package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

type recordingSender struct {
	email Email
	key   string
}

func (s *recordingSender) Send(_ context.Context, email Email, key string) (string, error) {
	s.email, s.key = email, key
	return "msg_test_42", nil
}

func TestCheckoutRequiresVerifiedEmail(t *testing.T) {
	tests := []struct {
		name      string
		verify    bool
		wantStage OrderStage
		wantErr   bool
	}{
		{name: "pending signup is blocked", wantStage: StageSignupPending, wantErr: true},
		{name: "verified signup enters checkout", verify: true, wantStage: StageCheckedOut},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &recordingSender{}
			workflow := NewStoreWorkflow(sender, "https://shop.example")
			workflow.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
			workflow.randomRead = func(p []byte) (int, error) { copy(p, strings.Repeat("a", len(p))); return len(p), nil }
			_, err := workflow.StartSignup(context.Background(), "cust-42", "buyer@example.com", "order-42")
			if err != nil {
				t.Fatal(err)
			}
			if sender.key != "signup-verification:cust-42" {
				t.Fatalf("idempotency key = %q", sender.key)
			}
			if tt.verify {
				token := strings.Repeat("61", 24)
				if _, err := workflow.Verify(token); err != nil {
					t.Fatal(err)
				}
			}
			got, err := workflow.Advance("cust-42", StageCheckedOut)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Advance() error = %v", err)
			}
			if tt.wantErr {
				got, _ = workflow.Lookup("cust-42")
			}
			if got.Stage != tt.wantStage {
				t.Fatalf("stage = %q, want %q", got.Stage, tt.wantStage)
			}
		})
	}
}
