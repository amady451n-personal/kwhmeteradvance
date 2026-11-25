package domain

import (
	"math"
	"testing"
	"time"

	"meteran/internal/store"
)

func reading(id, ts string, kwh float64, recharge *float64) store.Reading {
	return store.Reading{ID: id, Timestamp: ts, Reading: kwh, RechargeAmount: recharge}
}

func TestDailyUsageSplitsAcrossDays(t *testing.T) {
	rows := []store.Reading{
		reading("a", "2026-11-20T00:00:00+07:00", 100, nil),
		reading("b", "2026-11-22T00:00:00+07:00", 40, nil),
	}
	daily := DailyUsage(rows)
	if math.Abs(daily["2026-11-20"]-30) > 0.001 || math.Abs(daily["2026-11-21"]-30) > 0.001 {
		t.Fatalf("daily = %v", daily)
	}
	if _, ok := daily["2026-11-22"]; ok {
		t.Fatal("the end day holds no consumption of its own")
	}
}

func TestUsageBucketWeekAndMonthKeys(t *testing.T) {
	rows := []store.Reading{
		reading("a", "2026-11-16T00:00:00+07:00", 100, nil),
		reading("b", "2026-11-23T00:00:00+07:00", 30, nil),
	}
	from := ToMs("2026-11-16T00:00:00+07:00")
	to := ToMs("2026-11-24T00:00:00+07:00")
	buckets := UsageBuckets(rows, BucketInput{FromMs: from, ToMs: to, Group: "week"})
	if len(buckets) != 2 {
		t.Fatalf("buckets = %d", len(buckets))
	}
	if buckets[0].Key != "2026-11-16..2026-11-22" || math.Abs(buckets[0].Kwh-70) > 0.001 {
		t.Fatalf("week 1 = %+v", buckets[0])
	}
	if buckets[1].Key != "2026-11-23..2026-11-23" || math.Abs(buckets[1].Kwh-0) > 0.001 {
		t.Fatalf("week 2 = %+v", buckets[1])
	}
	if !buckets[0].Estimated {
		t.Fatal("days without their own reading are estimated")
	}
}

func TestSummarizeMatchesCalculations(t *testing.T) {
	rows := []store.Reading{
		reading("a", "2026-11-20T00:00:00+07:00", 100, nil),
		reading("b", "2026-11-22T00:00:00+07:00", 40, nil),
	}
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, store.WIB)
	sum := Summarize(SummaryInput{
		Readings: rows, Buffer: 10, AvgWindow: 7, Now: now, TargetHour: 6,
	})
	if sum.DataQuality != "ok" {
		t.Fatalf("quality = %s", sum.DataQuality)
	}
	if sum.AvgKwhPerDay == nil || math.Abs(*sum.AvgKwhPerDay-60.0/7) > 0.001 {
		t.Fatalf("avg = %v", sum.AvgKwhPerDay)
	}
	if sum.AvailableKwh != 40 || sum.UsableKwh != 30 {
		t.Fatalf("available=%v usable=%v", sum.AvailableKwh, sum.UsableKwh)
	}
	if sum.DaysLeft == nil || math.Abs(*sum.DaysLeft-3.5) > 0.001 {
		t.Fatalf("daysLeft = %v", sum.DaysLeft)
	}
	if len(sum.Chart) != 7 || sum.Chart[6].Key != "2026-11-22" {
		t.Fatalf("chart = %+v", sum.Chart)
	}
	// Monday 16 Nov to Sunday 22 Nov is the current week.
	if sum.WeekKwh == nil || math.Abs(*sum.WeekKwh-60) > 0.001 {
		t.Fatalf("week = %v", sum.WeekKwh)
	}
	if sum.NeedKwh <= 0 || sum.TargetAt != "2026-12-01T06:00:00+07:00" {
		t.Fatalf("need=%v target=%s", sum.NeedKwh, sum.TargetAt)
	}
}

func TestRecommendTokenPicksSmallestCoveringDenomination(t *testing.T) {
	price := 1500.0
	rec := RecommendToken(10, &price, []int64{20000, 50000, 100000})
	if rec == nil || rec.Status != "ok" || rec.Denomination == nil || *rec.Denomination != 20000 {
		t.Fatalf("rec = %+v", rec)
	}
	if rec := RecommendToken(1000, &price, []int64{20000}); rec.Status != "largest_only" {
		t.Fatalf("rec = %+v", rec)
	}
	if rec := RecommendToken(100, nil, nil); rec.Status != "price_unknown" {
		t.Fatalf("rec = %+v", rec)
	}
}