package report

import (
	"bytes"
	"testing"
	"time"

	"meteran/internal/domain"
	"meteran/internal/store"
)

func TestSummaryPDFRendersRangeAndRows(t *testing.T) {
	kwh := 182.61
	sum := domain.Summary{
		ComputedAt:    "2026-11-25T19:00:00+07:00",
		WindowDays:    7,
		TodayKwh:      &kwh,
		AvgKwhPerDay:  &kwh,
		LatestReading: &store.Reading{Reading: 182.61, Timestamp: "2026-11-25T06:00:00+07:00"},
	}
	rows := []store.Reading{
		{Reading: 200, Timestamp: "2026-11-23T06:00:00+07:00", RechargeAmount: ptr(66.0)},
		{Reading: 182.61, Timestamp: "2026-11-24T06:00:00+07:00"},
		{Reading: 170, Timestamp: "2026-11-25T06:00:00+07:00"},
	}
	pdf := SummaryPDF(sum, rows, "2026-11-01T00:00:00+07:00", "2026-12-01T00:00:00+07:00")
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatal("must be a PDF")
	}
	if !bytes.Contains(pdf, []byte("01 Nov 2026")) {
		t.Fatal("range must appear in the report")
	}
	if bytes.Contains(pdf, []byte("200.00 kWh")) {
		t.Fatal("top-up rows must be skipped")
	}
	if !bytes.Contains(pdf, []byte("182.61 kWh")) || !bytes.Contains(pdf, []byte("170.00 kWh")) {
		t.Fatal("last readings must appear")
	}
	_ = time.Now
}

func ptr[T any](v T) *T { return &v }