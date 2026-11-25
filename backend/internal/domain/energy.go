// Package domain holds the calculations the server owns. It is a 1:1 port of
// frontend/src/domain/energy.ts so both sides agree on every displayed number.
package domain

import (
	"math"
	"sort"
	"strings"
	"time"

	"meteran/internal/store"
)

const (
	DayMs       = 86_400_000
	WibOffsetMs = 7 * 3_600_000
)

var defaultDenominations = store.DefaultDenominations

func ToMs(s string) int64 {
	t, err := store.ParseTime(s)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

func DayKey(ms int64) string {
	return time.UnixMilli(ms + WibOffsetMs).UTC().Format("2006-01-02")
}

func DayStart(ms int64) int64 {
	return int64(math.Floor(float64(ms+WibOffsetMs)/DayMs))*DayMs - WibOffsetMs
}

var dayNames = []string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"}
var monthNames = []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func DayLabel(key string) string {
	t, err := store.ParseTime(key)
	if err != nil {
		return key
	}
	return dayNames[int(t.In(store.WIB).Weekday())]
}

func shortDate(ms int64) string {
	t := time.UnixMilli(ms).In(store.WIB)
	return t.Format("2") + " " + monthNames[int(t.Month())-1] + " " + t.Format("2006")
}

func shortMonth(ms int64) string {
	t := time.UnixMilli(ms).In(store.WIB)
	return monthNames[int(t.Month())-1] + " " + t.Format("2006")
}

func MonthStart(ms int64) int64 {
	t := time.UnixMilli(ms).In(store.WIB)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, store.WIB).UnixMilli()
}

func MonthNext(ms int64) int64 {
	t := time.UnixMilli(ms).In(store.WIB)
	return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, store.WIB).UnixMilli()
}

func WeekStart(ms int64) int64 {
	start := DayStart(ms)
	weekday := int(time.UnixMilli(start + WibOffsetMs).UTC().Weekday())
	return start - int64((weekday+6)%7)*DayMs
}

func NextTargetMs(nowMs int64, hour int) int64 {
	t := time.UnixMilli(nowMs).In(store.WIB)
	thisMonth := time.Date(t.Year(), t.Month(), 1, hour, 0, 0, 0, store.WIB).UnixMilli()
	if nowMs < thisMonth {
		return thisMonth
	}
	return time.Date(t.Year(), t.Month()+1, 1, hour, 0, 0, 0, store.WIB).UnixMilli()
}

// Interval is the gap between two readings with the consumption it implies.
type Interval struct {
	From        store.Reading
	To          store.Reading
	StartMs     int64
	EndMs       int64
	Hours       float64
	Consumption float64
	IsTopup     bool
}

func SortReadings(readings []store.Reading) []store.Reading {
	out := append([]store.Reading(nil), readings...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := ToMs(out[i].Timestamp), ToMs(out[j].Timestamp)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func BuildIntervals(readings []store.Reading) []Interval {
	sorted := SortReadings(readings)
	var result []Interval
	for i := 1; i < len(sorted); i++ {
		from, to := sorted[i-1], sorted[i]
		startMs, endMs := ToMs(from.Timestamp), ToMs(to.Timestamp)
		hours := float64(endMs-startMs) / 3_600_000
		topup := 0.0
		if to.RechargeAmount != nil {
			topup = *to.RechargeAmount
		}
		consumption := from.Reading + topup - to.Reading
		if hours <= 0 || consumption < 0 {
			continue
		}
		result = append(result, Interval{
			From: from, To: to, StartMs: startMs, EndMs: endMs,
			Hours: hours, Consumption: consumption, IsTopup: topup > 0,
		})
	}
	return result
}

// DailyUsage spreads each interval's consumption proportionally across the
// days it spans, so a missing day is an estimate rather than a gap (FR-22).
func DailyUsage(readings []store.Reading) map[string]float64 {
	daily := map[string]float64{}
	for _, iv := range BuildIntervals(readings) {
		if iv.Consumption <= 0 {
			continue
		}
		span := float64(iv.EndMs - iv.StartMs)
		for cursor := iv.StartMs; cursor < iv.EndMs; {
			boundary := DayStart(cursor) + DayMs
			segmentEnd := boundary
			if iv.EndMs < segmentEnd {
				segmentEnd = iv.EndMs
			}
			share := iv.Consumption * (float64(segmentEnd-cursor) / span)
			daily[DayKey(cursor)] += share
			cursor = segmentEnd
		}
	}
	return daily
}

func WindowUsage(daily map[string]float64, nowMs int64, days int) float64 {
	var total float64
	today := DayStart(nowMs)
	for i := 0; i < days; i++ {
		total += daily[DayKey(today-int64(i)*DayMs)]
	}
	return total
}

func EffectivePrice(readings []store.Reading) *float64 {
	var kwh, cost float64
	for _, r := range readings {
		recharge, paid := 0.0, int64(0)
		if r.RechargeAmount != nil {
			recharge = *r.RechargeAmount
		}
		if r.Cost != nil {
			paid = *r.Cost
		}
		if recharge > 0 && paid > 1 {
			kwh += recharge
			cost += float64(paid)
		}
	}
	if kwh <= 0 {
		return nil
	}
	v := cost / kwh
	return &v
}

// Recommendation is the token suggestion in the summary.
type Recommendation struct {
	Kwh          float64 `json:"kwh"`
	Rupiah       *int64  `json:"rupiah"`
	Denomination *int64  `json:"denomination"`
	Status       string  `json:"status"`
}

func RecommendToken(needKwh float64, price *float64, denominations []int64) *Recommendation {
	if needKwh <= 0 {
		return nil
	}
	if price == nil || *price <= 0 {
		return &Recommendation{Kwh: needKwh, Status: "price_unknown"}
	}
	needRupiah := needKwh * *price
	var sorted []int64
	for _, v := range denominations {
		if v > 0 {
			sorted = append(sorted, v)
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	if len(sorted) == 0 {
		return &Recommendation{Kwh: needKwh, Status: "no_denomination_available"}
	}
	for _, v := range sorted {
		if float64(v) >= needRupiah {
			kwh, denom := float64(v)/ *price, v
			return &Recommendation{Kwh: kwh, Rupiah: &denom, Denomination: &denom, Status: "ok"}
		}
	}
	largest := sorted[len(sorted)-1]
	kwh := float64(largest) / *price
	return &Recommendation{Kwh: kwh, Rupiah: &largest, Denomination: &largest, Status: "largest_only"}
}

type TariffComparison struct {
	PurchasePricePerKwh *float64 `json:"purchasePricePerKwh"`
	OfficialTariffPerKwh *float64 `json:"officialTariffPerKwh"`
	DiffRupiah          *float64 `json:"diffRupiah"`
	DiffPercent         *float64 `json:"diffPercent"`
}

func CompareTariff(purchase, official *float64) TariffComparison {
	out := TariffComparison{PurchasePricePerKwh: purchase, OfficialTariffPerKwh: official}
	if purchase == nil || official == nil || *official <= 0 {
		return out
	}
	diff := *purchase - *official
	percent := diff / *official * 100
	out.DiffRupiah, out.DiffPercent = &diff, &percent
	return out
}

type Urgency struct {
	Level               string   `json:"level"`
	HoursSinceLastInput *float64 `json:"hoursSinceLastInput"`
	Message             string   `json:"message"`
}

func UrgencyFor(latestMs *int64, nowMs int64) Urgency {
	if latestMs == nil {
		return Urgency{Level: "overdue", Message: "Belum ada catatan meter."}
	}
	hours := math.Max(0, float64(nowMs-*latestMs)/3_600_000)
	switch {
	case hours < 24:
		return Urgency{Level: "neutral", HoursSinceLastInput: &hours, Message: "Catatan terakhir masih baru."}
	case hours <= 40:
		return Urgency{Level: "remind", HoursSinceLastInput: &hours, Message: "Catat sekarang agar pemakaian tetap terpantau."}
	default:
		return Urgency{Level: "overdue", HoursSinceLastInput: &hours, Message: "Perlu input! Sisa kWh tidak terpantau."}
	}
}

type Alert struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// UsageBucket is the contract's aggregated usage row.
type UsageBucket struct {
	Key           string  `json:"key"`
	Label         string  `json:"label"`
	Start         string  `json:"start"`
	End           string  `json:"end"`
	Kwh           float64 `json:"kwh"`
	Cost          *int64  `json:"cost"`
	Estimated     bool    `json:"estimated"`
	EstimatedDays int     `json:"estimatedDays"`
	Days          int     `json:"days"`
}

type Summary struct {
	ComputedAt           string           `json:"computedAt"`
	WindowDays           int              `json:"windowDays"`
	DataQuality          string           `json:"dataQuality"`
	LatestReading        *store.Reading   `json:"latestReading"`
	AvgKwhPerDay         *float64         `json:"avgKwhPerDay"`
	TodayKwh             *float64         `json:"todayKwh"`
	WeekKwh              *float64         `json:"weekKwh"`
	WeekCost             *int64           `json:"weekCost"`
	MonthKwh             *float64         `json:"monthKwh"`
	MonthCost            *int64           `json:"monthCost"`
	EffectivePricePerKwh *float64         `json:"effectivePricePerKwh"`
	Buffer               float64          `json:"buffer"`
	TokenStockKwh        float64          `json:"tokenStockKwh"`
	AvailableKwh         float64          `json:"availableKwh"`
	UsableKwh            float64          `json:"usableKwh"`
	DaysLeft             *float64         `json:"daysLeft"`
	DepletionAt          *string          `json:"depletionAt"`
	TargetAt             string           `json:"targetAt"`
	NeedKwh              float64          `json:"needKwh"`
	NeedRupiah           *int64           `json:"needRupiah"`
	Recommendation       *Recommendation  `json:"recommendation"`
	TariffComparison     TariffComparison `json:"tariffComparison"`
	Urgency              Urgency          `json:"urgency"`
	Alerts               []Alert          `json:"alerts"`
	Chart                []UsageBucket    `json:"chart"`
}

// SummaryInput is everything Summarize needs; the handler fills it from the DB.
type SummaryInput struct {
	Readings       []store.Reading
	Tokens         []store.Token
	Buffer         float64
	AvgWindow      int
	Now            time.Time
	Denominations  []int64
	TargetHour     int
	OfficialTariff *float64
}

func Summarize(in SummaryInput) Summary {
	nowMs := in.Now.UnixMilli()
	daily := DailyUsage(in.Readings)
	sorted := SortReadings(in.Readings)
	var latest *store.Reading
	if len(sorted) > 0 {
		latest = &sorted[len(sorted)-1]
	}
	todayKey := DayKey(nowMs)
	monthKey := todayKey[:7]
	avgKwh := WindowUsage(daily, nowMs, in.AvgWindow) / math.Max(1, float64(in.AvgWindow))

	var tokenStock float64
	for _, t := range in.Tokens {
		if t.Status == "unused" && t.Type == "additional" {
			tokenStock += t.Kwh
		}
	}
	availableKwh := tokenStock
	if latest != nil {
		availableKwh += latest.Reading
	}
	usableKwh := math.Max(0, availableKwh-in.Buffer)

	var daysLeft *float64
	if latest != nil && avgKwh > 0 {
		v := usableKwh / avgKwh
		daysLeft = &v
	}
	targetMs := NextTargetMs(nowMs, in.TargetHour)
	daysToTarget := float64(targetMs-nowMs) / DayMs
	needKwh := 0.0
	if latest != nil {
		needKwh = math.Max(0, avgKwh*daysToTarget+in.Buffer-availableKwh)
	}
	price := EffectivePrice(in.Readings)
	denominations := in.Denominations
	if denominations == nil {
		denominations = defaultDenominations
	}
	quality := "insufficient"
	if latest != nil && len(sorted) >= 2 && avgKwh > 0 {
		quality = "ok"
	}

	var monthKwh float64
	for key, v := range daily {
		if strings.HasPrefix(key, monthKey) {
			monthKwh += v
		}
	}
	var weekKwh float64
	for cursor := WeekStart(nowMs); cursor <= DayStart(nowMs); cursor += DayMs {
		weekKwh += daily[DayKey(cursor)]
	}

	var recommendation *Recommendation
	if quality == "ok" {
		recommendation = RecommendToken(needKwh, price, denominations)
	}
	var latestMs *int64
	if latest != nil {
		ms := ToMs(latest.Timestamp)
		latestMs = &ms
	}
	urg := UrgencyFor(latestMs, nowMs)

	var needRupiah *int64
	if recommendation != nil && recommendation.Rupiah != nil {
		needRupiah = recommendation.Rupiah
	} else if price != nil {
		v := int64(math.Round(needKwh * *price))
		needRupiah = &v
	}

	chart := make([]UsageBucket, 0, 7)
	today := DayStart(nowMs)
	for i := 6; i >= 0; i-- {
		key := DayKey(today - int64(i)*DayMs)
		start := DayStart(today - int64(i)*DayMs)
		b := UsageBucket{
			Key: key, Label: DayLabel(key), Start: store.FormatTime(time.UnixMilli(start)),
			End: store.FormatTime(time.UnixMilli(start + DayMs)), Kwh: daily[key], Days: 1,
		}
		chart = append(chart, b)
	}

	s := Summary{
		ComputedAt:           store.FormatTime(in.Now),
		WindowDays:           in.AvgWindow,
		DataQuality:          quality,
		LatestReading:        latest,
		AvgKwhPerDay:         &avgKwh,
		WeekKwh:              &weekKwh,
		MonthKwh:             &monthKwh,
		EffectivePricePerKwh: price,
		Buffer:               in.Buffer,
		TokenStockKwh:        tokenStock,
		AvailableKwh:         availableKwh,
		UsableKwh:            usableKwh,
		DaysLeft:             daysLeft,
		TargetAt:             store.FormatTime(time.UnixMilli(targetMs)),
		NeedKwh:              needKwh,
		NeedRupiah:           needRupiah,
		Recommendation:       recommendation,
		TariffComparison:     CompareTariff(price, in.OfficialTariff),
		Urgency:              urg,
		Chart:                chart,
	}
	todayUsage := daily[todayKey]
	s.TodayKwh = &todayUsage
	if price != nil {
		wc := int64(math.Round(weekKwh * *price))
		mc := int64(math.Round(monthKwh * *price))
		s.WeekCost, s.MonthCost = &wc, &mc
	}
	if daysLeft != nil {
		dep := store.FormatTime(time.UnixMilli(nowMs + int64(*daysLeft*DayMs)))
		s.DepletionAt = &dep
	}
	s.Alerts = buildAlerts(quality, daysLeft, urg, tokenStock, recommendation, denominations)
	return s
}

func buildAlerts(quality string, daysLeft *float64, urg Urgency, tokenStock float64, rec *Recommendation, denominations []int64) []Alert {
	alerts := []Alert{}
	if quality == "insufficient" {
		alerts = append(alerts, Alert{"data_insufficient", "info", "Butuh minimal dua catatan untuk memprediksi pemakaian."})
	}
	if urg.Level == "remind" {
		alerts = append(alerts, Alert{"input_overdue", "warning", urg.Message})
	}
	if urg.Level == "overdue" {
		alerts = append(alerts, Alert{"input_overdue", "critical", urg.Message})
	}
	if daysLeft != nil && *daysLeft <= 3 {
		severity := "warning"
		if *daysLeft <= 1 {
			severity = "critical"
		}
		alerts = append(alerts, Alert{"buffer_near", severity, "Sisa kWh mendekati buffer."})
	}
	if rec != nil && tokenStock <= 0 {
		alerts = append(alerts, Alert{"topup_reminder", "warning", "Belum ada token tersimpan."})
	}
	if len(denominations) == 0 {
		alerts = append(alerts, Alert{"denomination_missing", "info", "Daftar denominasi kosong, saran token tanpa nominal."})
	}
	return alerts
}

type BucketInput struct {
	FromMs int64
	ToMs   int64
	Group  string
	Price  *float64
}

// UsageBuckets groups the daily estimates into day/week/month rows.
func UsageBuckets(readings []store.Reading, in BucketInput) []UsageBucket {
	daily := DailyUsage(readings)
	ownDays := map[string]bool{}
	for _, r := range readings {
		ownDays[DayKey(ToMs(r.Timestamp))] = true
	}
	out := []UsageBucket{}
	for start := bucketStart(in.FromMs, in.Group); start < in.ToMs; start = bucketNext(start, in.Group) {
		end := bucketNext(start, in.Group)
		if end > in.ToMs {
			end = in.ToMs
		}
		var kwh float64
		days, estimatedDays := 0, 0
		floor := start
		if ds := DayStart(in.FromMs); ds > floor {
			floor = ds
		}
		for cursor := floor; cursor < end; cursor += DayMs {
			key := DayKey(cursor)
			v := daily[key]
			kwh += v
			days++
			if v > 0 && !ownDays[key] {
				estimatedDays++
			}
		}
		key := DayKey(start)
		if in.Group == "month" {
			key = key[:7]
		} else if in.Group == "week" {
			key = DayKey(start) + ".." + DayKey(end-1)
		}
		labelStart := start
		if in.Group == "week" && DayStart(in.FromMs) > labelStart {
			labelStart = DayStart(in.FromMs)
		}
		var label string
		switch in.Group {
		case "day":
			label = shortDate(start)
		case "month":
			label = shortMonth(start)
		default:
			label = shortDate(labelStart) + " - " + shortDate(end-1)
		}
		b := UsageBucket{
			Key: key, Label: label,
			Start: store.FormatTime(time.UnixMilli(start)),
			End:   store.FormatTime(time.UnixMilli(end)),
			Kwh:   kwh, Estimated: estimatedDays > 0, EstimatedDays: estimatedDays, Days: days,
		}
		if in.Price != nil {
			c := int64(math.Round(kwh * *in.Price))
			b.Cost = &c
		}
		out = append(out, b)
	}
	return out
}

func bucketStart(ms int64, group string) int64 {
	switch group {
	case "week":
		return WeekStart(ms)
	case "month":
		return MonthStart(ms)
	default:
		return DayStart(ms)
	}
}

func bucketNext(ms int64, group string) int64 {
	switch group {
	case "week":
		return ms + 7*DayMs
	case "month":
		return MonthNext(ms)
	default:
		return ms + DayMs
	}
}