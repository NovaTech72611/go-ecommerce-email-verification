package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/url"
	"sync"
	"time"
)

type OrderStage string

const (
	StageSignupPending OrderStage = "signup_pending"
	StageEmailVerified OrderStage = "email_verified"
	StageCheckedOut    OrderStage = "checked_out"
	StageFulfilled     OrderStage = "fulfilled"
	StageReceiptSent   OrderStage = "receipt_sent"
	StageUpdated       OrderStage = "customer_updated"
)

type Signup struct {
	CustomerID string     `json:"customer_id"`
	Email      string     `json:"email"`
	OrderID    string     `json:"order_id"`
	Stage      OrderStage `json:"stage"`
	MessageID  string     `json:"message_id,omitempty"`
	tokenHash  [32]byte
	expiresAt  time.Time
}

type StoreWorkflow struct {
	mu         sync.Mutex
	signups    map[string]*Signup
	email      EmailSender
	publicURL  string
	now        func() time.Time
	randomRead func([]byte) (int, error)
}

func NewStoreWorkflow(sender EmailSender, publicURL string) *StoreWorkflow {
	return &StoreWorkflow{signups: make(map[string]*Signup), email: sender, publicURL: publicURL, now: time.Now, randomRead: rand.Read}
}

func (w *StoreWorkflow) StartSignup(ctx context.Context, customerID, email, orderID string) (Signup, error) {
	if customerID == "" || email == "" || orderID == "" {
		return Signup{}, errors.New("customer_id, email, and order_id are required")
	}
	raw := make([]byte, 24)
	if _, err := w.randomRead(raw); err != nil {
		return Signup{}, fmt.Errorf("create verification token: %w", err)
	}
	token := hex.EncodeToString(raw)
	link := w.publicURL + "/verify?token=" + url.QueryEscape(token)
	messageID, err := w.email.Send(ctx, Email{
		To: email, Subject: "Verify your email for checkout",
		HTML: `<p>Confirm your email to continue checkout.</p><p><a href="` + html.EscapeString(link) + `">Verify email</a></p>`,
	}, "signup-verification:"+customerID)
	if err != nil {
		return Signup{}, err
	}
	signup := &Signup{CustomerID: customerID, Email: email, OrderID: orderID, Stage: StageSignupPending, MessageID: messageID, tokenHash: sha256.Sum256([]byte(token)), expiresAt: w.now().Add(30 * time.Minute)}
	w.mu.Lock()
	w.signups[customerID] = signup
	w.mu.Unlock()
	return *signup, nil
}

func (w *StoreWorkflow) Verify(token string) (Signup, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	hash := sha256.Sum256([]byte(token))
	for _, signup := range w.signups {
		if signup.tokenHash == hash {
			if w.now().After(signup.expiresAt) {
				return Signup{}, errors.New("verification link expired")
			}
			signup.Stage = StageEmailVerified
			return *signup, nil
		}
	}
	return Signup{}, errors.New("verification link invalid")
}

func (w *StoreWorkflow) Advance(customerID string, next OrderStage) (Signup, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	signup, ok := w.signups[customerID]
	if !ok {
		return Signup{}, errors.New("signup not found")
	}
	allowed := map[OrderStage]OrderStage{StageEmailVerified: StageCheckedOut, StageCheckedOut: StageFulfilled, StageFulfilled: StageReceiptSent, StageReceiptSent: StageUpdated}
	if allowed[signup.Stage] != next {
		return Signup{}, fmt.Errorf("cannot move order from %s to %s", signup.Stage, next)
	}
	signup.Stage = next
	return *signup, nil
}

func (w *StoreWorkflow) Lookup(customerID string) (Signup, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	signup, ok := w.signups[customerID]
	if !ok {
		return Signup{}, false
	}
	return *signup, true
}
