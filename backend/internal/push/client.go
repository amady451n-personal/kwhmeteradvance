package push

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// ErrGone means the browser dropped the subscription; the row can be deleted.
var ErrGone = errors.New("push subscription gone")

// Subscriber is the minimal data Send needs from the store.
type Subscriber struct {
	ID       string
	Endpoint string
	P256dh   string
	Auth     string
}

// Client posts encrypted notifications to push services.
type Client struct {
	VAPID *VAPID
	HTTP  *http.Client
	TTL   int
}

// Send delivers one payload. Returns ErrGone when the endpoint is expired.
func (c *Client) Send(ctx context.Context, sub Subscriber, payload []byte) error {
	token, err := c.VAPID.Token(sub.Endpoint, time.Now())
	if err != nil {
		return err
	}
	body, err := Encrypt(sub.P256dh, sub.Auth, payload)
	if err != nil {
		return err
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = 3600
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", strconv.Itoa(ttl))
	req.Header.Set("Urgency", "normal")
	req.Header.Set("Authorization", "vapid t="+token+", k="+c.VAPID.Public)

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return ErrGone
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("push %s: %s: %s", sub.Endpoint, resp.Status, bytes.TrimSpace(snippet))
	}
	return nil
}
