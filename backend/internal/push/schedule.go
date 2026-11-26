package push

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"meteran/internal/domain"
	"meteran/internal/store"
)

// wib is the meter's wall clock: every date shown in the app is Asia/Jakarta.
var wib = time.FixedZone("WIB", 7*3600)

// DayWIB is the WIB calendar date used by push_log.
func DayWIB(t time.Time) string { return t.In(wib).Format("2006-01-02") }

// ReminderDue decides one tick: after the reminder hour, once per WIB day, and
// only when the meter reading is at least a day old.
func ReminderDue(now time.Time, reminderHour int, latestMs *int64, lastPushDay string) bool {
	if reminderHour < 0 {
		return false
	}
	if now.In(wib).Hour() < reminderHour {
		return false
	}
	today := DayWIB(now)
	if lastPushDay == today {
		return false
	}
	if latestMs != nil && now.UnixMilli()-*latestMs < domain.DayMs {
		return false
	}
	return true
}

// Payload is the notification body the service worker shows.
func Payload(latestAt string) []byte {
	body := "Belum ada catatan meter. Isi angka meter terbaru."
	if latestAt != "" {
		if ms := domain.ToMs(latestAt); ms > 0 {
			latestAt = time.UnixMilli(ms).In(wib).Format("02 Jan 2006")
		}
		body = "Catatan meter terakhir " + latestAt + ". Isi angka meter terbaru."
	}
	data, _ := json.Marshal(map[string]string{
		"title": "Waktunya catat meter",
		"body":  body,
		"url":   "/",
	})
	return data
}

// RunDaily checks once per minute and sends at most one reminder per WIB day.
// It returns when ctx is cancelled.
func (c *Client) RunDaily(ctx context.Context, db *store.DB, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := c.Tick(ctx, db, time.Now()); err != nil {
			log.Printf("push: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tick performs one reminder check. Exported for tests.
func (c *Client) Tick(ctx context.Context, db *store.DB, now time.Time) error {
	settings, err := db.GetSettings()
	if err != nil {
		return err
	}
	lastDay, err := db.LastPushDay()
	if err != nil {
		return err
	}
	latestAt, err := db.LatestReadingAt()
	if err != nil {
		return err
	}
	var latestMs *int64
	if latestAt != "" {
		ms := domain.ToMs(latestAt)
		latestMs = &ms
	}
	if !ReminderDue(now, settings.ReminderHour, latestMs, lastDay) {
		return nil
	}
	subs, err := db.ListSubscriptions()
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}
	payload := Payload(latestAt)
	sent := 0
	var lastErr error
	for _, sub := range subs {
		err := c.Send(ctx, Subscriber{ID: sub.ID, Endpoint: sub.Endpoint, P256dh: sub.Keys.P256dh, Auth: sub.Keys.Auth}, payload)
		if errors.Is(err, ErrGone) {
			if delErr := db.Unsubscribe(sub.ID); delErr != nil && !errors.Is(delErr, store.ErrNotFound) {
				log.Printf("push: drop %s: %v", sub.ID, delErr)
			}
			continue
		}
		if err != nil {
			lastErr = err
			continue
		}
		sent++
	}
	if sent == 0 {
		return lastErr
	}
	if err := db.MarkPushDay(DayWIB(now)); err != nil {
		return err
	}
	if count, err := db.CountSubscriptions(); err == nil && count == 0 {
		_ = db.SetPushEnabled(false)
	}
	return lastErr
}
