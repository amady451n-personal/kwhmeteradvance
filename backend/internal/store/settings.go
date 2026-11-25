package store

import (
	"database/sql"
	"encoding/json"
)

// DefaultDenominations mirrors the PRD's default voucher list.
var DefaultDenominations = []int64{20000, 50000, 100000, 200000, 500000}

// Tariff is only a comparison figure (FR-33); it never drives calculations.
type Tariff struct {
	Golongan *string  `json:"golongan"`
	PerKwh   *float64 `json:"perKwh"`
}

// Settings is the single settings row plus derived flags.
type Settings struct {
	Buffer            float64  `json:"buffer"`
	AvgWindow         int      `json:"avgWindow"`
	TargetHour        int      `json:"targetHour"`
	ReminderHour      int      `json:"reminderHour"`
	Denominations     []int64  `json:"denominations"`
	Tariff            Tariff   `json:"tariff"`
	PinSet            bool     `json:"pinSet"`
	PushEnabled       bool     `json:"pushEnabled"`
	ImportCompletedAt *string  `json:"importCompletedAt"`
	UpdatedAt         string   `json:"updatedAt"`
}

// SettingsPatch leaves omitted fields unchanged.
type SettingsPatch struct {
	Buffer        *float64  `json:"buffer"`
	AvgWindow     *int      `json:"avgWindow"`
	TargetHour    *int      `json:"targetHour"`
	ReminderHour  *int      `json:"reminderHour"`
	Denominations *[]int64  `json:"denominations"`
	Tariff        *Tariff   `json:"tariff"`
}

func (db *DB) GetSettings() (Settings, error) {
	var s Settings
	var raw string
	var golongan sql.NullString
	var perKwh sql.NullFloat64
	var push int
	var importAt sql.NullString
	err := db.SQL.QueryRow(`select buffer, avg_window, target_hour, reminder_hour, denominations,
		tariff_golongan, tariff_per_kwh, push_enabled, import_completed_at, updated_at
		from settings where id = 1`).Scan(&s.Buffer, &s.AvgWindow, &s.TargetHour, &s.ReminderHour,
		&raw, &golongan, &perKwh, &push, &importAt, &s.UpdatedAt)
	if isNoRows(err) {
		if err := db.resetSettings(); err != nil {
			return Settings{}, err
		}
		return db.GetSettings()
	}
	if err != nil {
		return Settings{}, err
	}
	_ = json.Unmarshal([]byte(raw), &s.Denominations)
	if s.Denominations == nil {
		s.Denominations = []int64{}
	}
	if golongan.Valid {
		s.Tariff.Golongan = &golongan.String
	}
	if perKwh.Valid {
		s.Tariff.PerKwh = &perKwh.Float64
	}
	s.PushEnabled = push == 1
	if importAt.Valid {
		s.ImportCompletedAt = &importAt.String
	}
	hasPin, err := db.HasPin()
	if err != nil {
		return Settings{}, err
	}
	s.PinSet = hasPin
	return s, nil
}

func (db *DB) resetSettings() error {
	raw, _ := json.Marshal(DefaultDenominations)
	_, err := db.SQL.Exec(`insert into settings
		(id, buffer, avg_window, target_hour, reminder_hour, denominations, updated_at)
		values (1, 10, 7, 6, 8, ?, ?)
		on conflict (id) do nothing`, string(raw), Now())
	return err
}

func (db *DB) PatchSettings(p SettingsPatch) (Settings, error) {
	if _, err := db.GetSettings(); err != nil { // ensures the row exists
		return Settings{}, err
	}
	set := []string{"updated_at = ?"}
	args := []any{Now()}
	if p.Buffer != nil {
		if *p.Buffer < 0 {
			return Settings{}, validation("buffer", "negative_value", "Buffer tidak boleh negatif.")
		}
		set = append(set, "buffer = ?")
		args = append(args, *p.Buffer)
	}
	if p.AvgWindow != nil {
		if *p.AvgWindow != 7 && *p.AvgWindow != 14 && *p.AvgWindow != 30 {
			return Settings{}, validation("avgWindow", "invalid_value", "Jendela rata-rata harus 7, 14, atau 30 hari.")
		}
		set = append(set, "avg_window = ?")
		args = append(args, *p.AvgWindow)
	}
	for field, hour := range map[string]*int{"targetHour": p.TargetHour, "reminderHour": p.ReminderHour} {
		if hour == nil {
			continue
		}
		if *hour < 0 || *hour > 23 {
			return Settings{}, validation(field, "invalid_value", "Jam harus 0 sampai 23.")
		}
		col := map[string]string{"targetHour": "target_hour", "reminderHour": "reminder_hour"}[field]
		set = append(set, col+" = ?")
		args = append(args, *hour)
	}
	if p.Denominations != nil {
		for _, d := range *p.Denominations {
			if d < 0 {
				return Settings{}, validation("denominations", "negative_value", "Denominasi tidak boleh negatif.")
			}
		}
		raw, err := json.Marshal(*p.Denominations)
		if err != nil {
			return Settings{}, err
		}
		set = append(set, "denominations = ?")
		args = append(args, string(raw))
	}
	if p.Tariff != nil {
		set = append(set, "tariff_golongan = ?", "tariff_per_kwh = ?")
		args = append(args, p.Tariff.Golongan, p.Tariff.PerKwh)
	}
	if _, err := db.SQL.Exec(`update settings set `+joinComma(set)+` where id = 1`, args...); err != nil {
		return Settings{}, err
	}
	return db.GetSettings()
}

// MarkImported records a successful legacy import (FR-105).
func (db *DB) MarkImported() error {
	_, err := db.SQL.Exec(`update settings set import_completed_at = ?, updated_at = ? where id = 1`, Now(), Now())
	return err
}

// SetPushEnabled mirrors whether at least one push subscription exists.
func (db *DB) SetPushEnabled(on bool) error {
	v := 0
	if on {
		v = 1
	}
	_, err := db.SQL.Exec(`update settings set push_enabled = ? where id = 1`, v)
	return err
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}