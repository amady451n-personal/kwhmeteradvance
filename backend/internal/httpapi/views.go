package httpapi

import (
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"meteran/internal/domain"
	"meteran/internal/store"
)

func (s *Server) summaryInput(r *http.Request) (domain.SummaryInput, error) {
	settings, err := s.DB.GetSettings()
	if err != nil {
		return domain.SummaryInput{}, err
	}
	readings, err := s.DB.AllReadings()
	if err != nil {
		return domain.SummaryInput{}, err
	}
	from, to, err := store.NormalizeRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		return domain.SummaryInput{}, err
	}
	if from != "" || to != "" {
		filtered := make([]store.Reading, 0, len(readings))
		for _, row := range readings {
			if (from == "" || row.Timestamp >= from) && (to == "" || row.Timestamp < to) {
				filtered = append(filtered, row)
			}
		}
		readings = filtered
	}
	tokens, _, err := s.DB.ListTokens(store.TokenQuery{Limit: 200, Status: "unused"})
	if err != nil {
		return domain.SummaryInput{}, err
	}
	return domain.SummaryInput{
		Readings:       readings,
		Tokens:         tokens,
		Buffer:         settings.Buffer,
		AvgWindow:      settings.AvgWindow,
		Now:            time.Now(),
		Denominations:  settings.Denominations,
		TargetHour:     settings.TargetHour,
		OfficialTariff: settings.Tariff.PerKwh,
	}, nil
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	in, err := s.summaryInput(r)
	if err != nil {
		failStore(w, err)
		return
	}
	switch raw := r.URL.Query().Get("window"); raw {
	case "":
	case "7", "14", "30":
		in.AvgWindow = atoiDefault(raw, in.AvgWindow)
	default:
		Fail(w, http.StatusUnprocessableEntity, CodeValidationError, "Data tidak valid.",
			Detail{Field: "window", Code: "invalid_value", Message: "Jendela harus 7, 14, atau 30."})
		return
	}
	Data(w, http.StatusOK, domain.Summarize(in))
}

func (s *Server) usage(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	if group == "" {
		group = "day"
	}
	if group != "day" && group != "week" && group != "month" {
		Fail(w, http.StatusUnprocessableEntity, CodeValidationError, "Data tidak valid.",
			Detail{Field: "group", Code: "invalid_value", Message: "Grup harus day, week, atau month."})
		return
	}
	from, to, err := store.NormalizeRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		failStore(w, err)
		return
	}
	now := time.Now()
	if to == "" {
		to = store.FormatTime(now)
	}
	if from == "" {
		from = store.FormatTime(now.AddDate(0, 0, -30))
	}
	readings, err := s.DB.AllReadings()
	if err != nil {
		failStore(w, err)
		return
	}
	fromMs := domain.ToMs(from)
	buckets := domain.UsageBuckets(readings, domain.BucketInput{
		FromMs: fromMs, ToMs: domain.ToMs(to), Group: group, Price: domain.EffectivePrice(readings),
	})

	// Newest period first: the report shows the latest days even on a long range.
	for i, j := 0, len(buckets)-1; i < j; i, j = i+1, j-1 {
		buckets[i], buckets[j] = buckets[j], buckets[i]
	}

	totalKwh := 0.0
	var totalCost int64
	for _, b := range buckets {
		totalKwh += b.Kwh
		if b.Cost != nil {
			totalCost += *b.Cost
		}
	}

	// Buckets are cheap to compute, so paging is a plain offset into the slice.
	limit := atoiDefault(r.URL.Query().Get("limit"), 20)
	if limit < 1 || limit > 200 {
		limit = 20
	}
	offset := decodeOffset(r.URL.Query().Get("cursor"))
	if offset > len(buckets) {
		offset = len(buckets)
	}
	end := offset + limit
	if end > len(buckets) {
		end = len(buckets)
	}
	page := buckets[offset:end]
	meta := map[string]any{
		"hasMore":   end < len(buckets),
		"total":     len(buckets),
		"totalKwh":  totalKwh,
		"group":     group,
		"nextCursor": nil,
	}
	if end < len(buckets) {
		meta["nextCursor"] = encodeOffset(end)
	}
	price := domain.EffectivePrice(readings)
	if price != nil {
		meta["totalCost"] = totalCost
	} else {
		meta["totalCost"] = nil
	}
	DataMeta(w, http.StatusOK, page, meta)
}

func encodeOffset(n int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("o:" + strconv.Itoa(n)))
}

func decodeOffset(c string) int {
	if c == "" {
		return 0
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimPrefix(string(b), "o:"))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

type homepageTarget struct {
	Name      string `json:"name"`
	Value     any    `json:"value"`
	Formatted string `json:"formatted"`
}

// homepage is the fixed FR-111 shape: no envelope, no auth.
func (s *Server) homepage(w http.ResponseWriter, r *http.Request) {
	in, err := s.summaryInput(r)
	if err != nil {
		failStore(w, err)
		return
	}
	sum := domain.Summarize(in)

	var value any
	textValue := "N/A"
	if sum.DepletionAt != nil {
		if ms := domain.ToMs(*sum.DepletionAt); ms > 0 {
			secs := float64(ms) / 1000
			value = secs
			textValue = homepageTime(*sum.DepletionAt)
		}
	}

	targets := []homepageTarget{
		{Name: "last_input_ago", Value: nil, Formatted: "N/A"},
		{Name: "last_kwh", Value: nil, Formatted: "N/A"},
		{Name: "last_kwh_per_day", Value: nil, Formatted: "N/A"},
		{Name: "estimated_depletion", Value: value, Formatted: textValue},
	}
	if sum.Urgency.HoursSinceLastInput != nil {
		h := round(*sum.Urgency.HoursSinceLastInput, 2)
		targets[0] = homepageTarget{Name: "last_input_ago", Value: h, Formatted: fmt.Sprintf("%.2fh ago", h)}
	}
	if sum.LatestReading != nil {
		kwh := round(sum.LatestReading.Reading, 2)
		targets[1] = homepageTarget{Name: "last_kwh", Value: kwh, Formatted: fmt.Sprintf("%.2f kWh", kwh)}
	}
	if sum.AvgKwhPerDay != nil {
		avg := round(*sum.AvgKwhPerDay, 2)
		targets[2] = homepageTarget{Name: "last_kwh_per_day", Value: avg, Formatted: fmt.Sprintf("%.2f kWh/day", avg)}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"label":     "kWh Tracker",
		"value":     value,
		"textValue": textValue,
		"targets":   targets,
	})
}

func round(v float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	return math.Round(v*factor) / factor
}

func homepageTime(iso string) string {
	t, err := store.ParseTime(iso)
	if err != nil {
		return "N/A"
	}
	return t.In(store.WIB).Format("02 Jan 2006, 03:04 PM") + " WIB"
}