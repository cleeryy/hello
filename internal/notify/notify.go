package notify

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// requestTimeout bounds one webhook delivery attempt. Slow receivers must
// never stall wake responses or the monitor loop.
const requestTimeout = 5 * time.Second

// Sender posts JSON payloads to one webhook URL. A nil client keeps the
// zero value usable; a blank URL disables delivery without error.
type Sender struct {
	url    string
	client *http.Client
}

// New returns a Sender for url. Blank means disabled.
func New(url string) *Sender {
	return &Sender{url: url, client: &http.Client{Timeout: requestTimeout}}
}

// Enabled reports whether deliveries will be attempted.
func (s *Sender) Enabled() bool {
	return s != nil && s.url != ""
}

// Send marshals payload and POSTs it in the background: webhook delivery
// never blocks the caller, and failures are only logged.
func (s *Sender) Send(payload any) {
	if !s.Enabled() {
		return
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("webhook encode failed", slog.String("url", s.url), slog.Any("err", err))
		return
	}
	go func() {
		resp, err := s.client.Post(s.url, "application/json", bytes.NewReader(raw))
		if err != nil {
			slog.Warn("webhook delivery failed", slog.String("url", s.url), slog.Any("err", err))
			return
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			slog.Warn("webhook rejected", slog.String("url", s.url), slog.Int("status", resp.StatusCode))
		}
	}()
}

// StatusPayload is posted when a device reachability changes.
type StatusPayload struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
	At       int64  `json:"at"`
}

// WakePayload is posted after each wake attempt batch.
type WakePayload struct {
	DeviceID string `json:"device_id"`
	MAC      string `json:"mac"`
	Success  bool   `json:"success"`
	Attempts int    `json:"attempts"`
	Trigger  string `json:"trigger"`
}
