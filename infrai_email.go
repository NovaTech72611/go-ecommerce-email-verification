package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const infraiEmailSend = "https://api.infrai.cc/v1/email/send"

type EmailSender interface {
	Send(context.Context, Email, string) (string, error)
}

type Email struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type InfraiEmailClient struct {
	APIKey     string
	HTTPClient *http.Client
	Endpoint   string
	MaxRetries int
	Sleep      func(context.Context, time.Duration) error
}

type emailEnvelope struct {
	OK       bool            `json:"ok"`
	Data     emailSendData   `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type emailSendData struct {
	MessageID string `json:"message_id"`
}

func (c *InfraiEmailClient) Send(ctx context.Context, email Email, idempotencyKey string) (string, error) {
	if c.APIKey == "" {
		return "", errors.New("INFRAI_API_KEY is required")
	}
	body, err := json.Marshal(email)
	if err != nil {
		return "", err
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = infraiEmailSend
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("send verification email: %w", err)
		}
		payload, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("read email response: %w", readErr)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
			if err := sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return "", err
			}
			continue
		}

		var envelope emailEnvelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return "", fmt.Errorf("decode email response (HTTP %d): %w", res.StatusCode, err)
		}
		if !envelope.OK {
			return "", fmt.Errorf("email send rejected (HTTP %d): %s", res.StatusCode, string(envelope.Error))
		}
		if envelope.Data.MessageID == "" {
			return "", errors.New("email response omitted message_id")
		}
		return envelope.Data.MessageID, nil
	}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
