package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"io"
	"math"
	"sort"
	"strings"
)

// Token is one prepaid voucher. The code is stored encrypted and only exposed
// by RevealTokenCode.
type Token struct {
	ID                   string  `json:"id"`
	Kwh                  float64 `json:"kwh"`
	Cost                 *int64  `json:"cost"`
	PurchasedAt          string  `json:"purchasedAt"`
	Type                 string  `json:"type"`
	Denomination         *int64  `json:"denomination"`
	DenominationMismatch bool    `json:"denominationMismatch"`
	Status               string  `json:"status"`
	Source               string  `json:"source"`
	Notes                *string `json:"notes"`
	HasCode              bool    `json:"hasCode"`
	LinkedReadingID      *string `json:"linkedReadingId"`
	CanDelete            bool    `json:"canDelete"`
	DeleteBlockedReason  *string `json:"deleteBlockedReason"`
	LockedAt             *string `json:"lockedAt"`
	RefillsSinceLock     *int    `json:"refillsSinceLock"`
	CreatedAt            string  `json:"createdAt"`
	UpdatedAt            string  `json:"updatedAt"`
}

// TokenInput is TokenCreate/TokenUpsert.
type TokenInput struct {
	ID           string   `json:"id"`
	Code         string   `json:"code"`
	Kwh          *float64 `json:"kwh"`
	Cost         *int64   `json:"cost"`
	PurchasedAt  string   `json:"purchasedAt"`
	Type         string   `json:"type"`
	Denomination *int64   `json:"denomination"`
	Notes        *string  `json:"notes"`
	Confirm      []string `json:"confirm"`
}

// TokenPatch only touches fields that are safe before a token is locked.
type TokenPatch struct {
	Type         *string  `json:"type"`
	Denomination *int64   `json:"denomination"`
	Notes        *string  `json:"notes"`
	PurchasedAt  *string  `json:"purchasedAt"`
}

// TokenQuery filters the token list.
type TokenQuery struct {
	Status string
	Limit  int
	Cursor string
}

const tokenCols = `id, kwh, cost, purchased_at, type, denomination, status, source, notes,
	code_enc is not null, linked_reading_id, locked_at, created_at, updated_at`

func scanToken(sc interface{ Scan(...any) error }) (Token, error) {
	var t Token
	var cost, denom sql.NullInt64
	var notes, linked, lockedAt sql.NullString
	var hasCode int
	err := sc.Scan(&t.ID, &t.Kwh, &cost, &t.PurchasedAt, &t.Type, &denom, &t.Status, &t.Source,
		&notes, &hasCode, &linked, &lockedAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Token{}, err
	}
	if cost.Valid {
		t.Cost = &cost.Int64
	}
	if denom.Valid {
		t.Denomination = &denom.Int64
	}
	if notes.Valid {
		t.Notes = &notes.String
	}
	if linked.Valid {
		t.LinkedReadingID = &linked.String
	}
	if lockedAt.Valid {
		t.LockedAt = &lockedAt.String
	}
	t.HasCode = hasCode == 1
	return t, nil
}

func (db *DB) ListTokens(q TokenQuery) ([]Token, PageMeta, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 20
	}
	where := []string{"1 = 1"}
	args := []any{}
	if q.Status != "" {
		where = append(where, "status = ?")
		args = append(args, q.Status)
	}
	if q.Cursor != "" {
		pa, id, ok := decodeCursor(q.Cursor)
		if ok {
			where = append(where, "(purchased_at < ? or (purchased_at = ? and id < ?))")
			args = append(args, pa, pa, id)
		}
	}
	args = append(args, q.Limit+1)
	rows, err := db.SQL.Query(`select `+tokenCols+` from tokens where `+strings.Join(where, " and ")+
		` order by purchased_at desc, id desc limit ?`, args...)
	if err != nil {
		return nil, PageMeta{}, err
	}
	defer rows.Close()
	out := []Token{}
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, PageMeta{}, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, PageMeta{}, err
	}
	meta := PageMeta{}
	if len(out) > q.Limit {
		out = out[:q.Limit]
		last := out[len(out)-1]
		c := encodeCursor(last.PurchasedAt, last.ID)
		meta.NextCursor = &c
		meta.HasMore = true
	}
	if err := db.decorateTokens(out); err != nil {
		return nil, PageMeta{}, err
	}
	return out, meta, nil
}

func (db *DB) GetToken(id string) (Token, error) {
	t, err := scanToken(db.SQL.QueryRow(`select `+tokenCols+` from tokens where id = ?`, id))
	if isNoRows(err) {
		return Token{}, ErrNotFound
	}
	if err != nil {
		return Token{}, err
	}
	list := []Token{t}
	if err := db.decorateTokens(list); err != nil {
		return Token{}, err
	}
	return list[0], nil
}

// decorateTokens fills derived fields. FR-13: a locked token may be deleted
// once 50 top-ups have been recorded after it locked.
func (db *DB) decorateTokens(list []Token) error {
	for i := range list {
		t := &list[i]
		if t.Status == "locked" && t.LockedAt != nil {
			var n int
			err := db.SQL.QueryRow(`select count(*) from readings
				where recharge_amount is not null and recharge_amount > 0 and timestamp > ?`, *t.LockedAt).Scan(&n)
			if err != nil {
				return err
			}
			t.RefillsSinceLock = &n
			t.CanDelete = n >= 50
			if !t.CanDelete {
				reason := "refills_pending"
				t.DeleteBlockedReason = &reason
			}
		} else {
			reason := "not_locked"
			t.DeleteBlockedReason = &reason
			t.CanDelete = false
		}
		t.DenominationMismatch = denominationMismatch(t.Cost, t.Denomination)
	}
	return nil
}

func denominationMismatch(cost, denomination *int64) bool {
	if cost == nil {
		return false
	}
	if denomination == nil {
		return true
	}
	return math.Abs(float64(*cost-*denomination)) > float64(*denomination)*0.1
}

// priceFor returns the nearest denomination to cost, or the computed price.
func priceFor(kwh float64, cost int64, denominations []int64) *int64 {
	if kwh <= 0 {
		return nil
	}
	if len(denominations) == 0 {
		return nil
	}
	sorted := append([]int64(nil), denominations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	best := sorted[0]
	for _, v := range sorted[1:] {
		if math.Abs(float64(v-cost)) < math.Abs(float64(best-cost)) {
			best = v
		}
	}
	return &best
}

func (db *DB) CreateToken(in TokenInput) (Token, error) {
	if in.ID == "" {
		in.ID = NewID("token")
	}
	return db.UpsertToken(in, true)
}

// UpsertToken is idempotent for offline replay: an identical payload on an
// existing token returns the stored row unchanged.
func (db *DB) UpsertToken(in TokenInput, requireCode bool) (Token, error) {
	if in.Kwh == nil || *in.Kwh <= 0 {
		return Token{}, validation("kwh", "invalid_kwh", "Jumlah kWh harus lebih dari nol.")
	}
	if in.Cost != nil && *in.Cost < 0 {
		return Token{}, validation("cost", "negative_value", "Biaya tidak boleh negatif.")
	}
	if in.Type != "additional" && in.Type != "bulk" {
		return Token{}, validation("type", "invalid_type", "Jenis token tidak dikenal.")
	}
	if in.PurchasedAt == "" {
		return Token{}, validation("purchasedAt", "required", "Waktu pembelian wajib diisi.")
	}
	ts, err := ParseTime(in.PurchasedAt)
	if err != nil {
		return Token{}, validation("purchasedAt", "invalid_timestamp", "Waktu pembelian tidak valid.")
	}
	stamp := FormatTime(ts)

	existing, err := db.GetToken(in.ID)
	if err != nil && err != ErrNotFound {
		return Token{}, err
	}
	exists := err == nil
	if exists {
		if existing.Status == "locked" {
			return Token{}, ErrTokenLocked
		}
		if sameToken(existing, in, stamp) {
			return existing, nil
		}
		if existing.Status == "used" {
			return Token{}, validation("id", "token_used", "Token yang sudah dipakai tidak bisa diubah.")
		}
	}
	if requireCode && in.Code == "" && !exists {
		return Token{}, validation("code", "required", "Kode token wajib diisi.")
	}
	if in.Code != "" && (len(in.Code) < 4 || len(in.Code) > 64) {
		return Token{}, validation("code", "invalid_code", "Kode token harus 4 sampai 64 karakter.")
	}

	denoms, err := db.denominations()
	if err != nil {
		return Token{}, err
	}
	denom := in.Denomination
	if denom == nil && in.Cost != nil {
		denom = priceFor(*in.Kwh, *in.Cost, denoms)
	}
	if err := confirmToken(db, in, stamp, denom); err != nil {
		return Token{}, err
	}

	now := Now()
	createdAt := now
	if exists {
		createdAt = existing.CreatedAt
	}
	var codeEnc []byte
	if in.Code != "" {
		if codeEnc, err = db.encrypt(in.Code); err != nil {
			return Token{}, err
		}
	} else {
		err = db.SQL.QueryRow(`select code_enc from tokens where id = ?`, in.ID).Scan(&codeEnc)
		if err != nil && err != sql.ErrNoRows {
			return Token{}, err
		}
	}
	_, err = db.SQL.Exec(`insert into tokens
		(id, kwh, cost, purchased_at, type, denomination, status, source, notes, code_enc, created_at, updated_at)
		values (?, ?, ?, ?, ?, ?, coalesce((select status from tokens where id = ?), 'unused'), 'manual', ?, ?, ?, ?)
		on conflict (id) do update set
			kwh = excluded.kwh, cost = excluded.cost, purchased_at = excluded.purchased_at,
			type = excluded.type, denomination = excluded.denomination, notes = excluded.notes,
			code_enc = coalesce(excluded.code_enc, tokens.code_enc), updated_at = excluded.updated_at`,
		in.ID, *in.Kwh, in.Cost, stamp, in.Type, denom, in.ID, in.Notes, codeEnc, createdAt, now)
	if err != nil {
		return Token{}, err
	}
	return db.GetToken(in.ID)
}

func sameToken(t Token, in TokenInput, stamp string) bool {
	return t.Kwh == *in.Kwh && t.PurchasedAt == stamp && t.Type == in.Type &&
		eqInt64(t.Cost, in.Cost) && in.Code == ""
}

func eqInt64(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func confirmToken(db *DB, in TokenInput, stamp string, denom *int64) error {
	confirmed := map[string]bool{}
	for _, c := range in.Confirm {
		confirmed[c] = true
	}
	var details []Detail
	if !confirmed[ConfirmDuplicateTimestamp] {
		var n int
		if err := db.SQL.QueryRow(`select count(*) from tokens where purchased_at = ? and id <> ?`, stamp, in.ID).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			details = append(details, Detail{Field: "purchasedAt", Code: ConfirmDuplicateTimestamp, Message: "Waktu pembelian sudah dipakai token lain."})
		}
	}
	if !confirmed[ConfirmDenominationOutOfTol] && denominationMismatch(in.Cost, denom) {
		details = append(details, Detail{Field: "cost", Code: ConfirmDenominationOutOfTol, Message: "Biaya di luar nominal token yang biasa. Pastikan tidak salah."})
	}
	if len(details) > 0 {
		return ErrConfirmation{Details: details}
	}
	return nil
}

func (db *DB) PatchToken(id string, p TokenPatch) (Token, error) {
	t, err := db.GetToken(id)
	if err != nil {
		return Token{}, err
	}
	if t.Status == "locked" {
		return Token{}, ErrTokenLocked
	}
	if t.Status == "used" {
		return Token{}, validation("id", "token_used", "Token yang sudah dipakai tidak bisa diubah.")
	}
	set := []string{"updated_at = ?"}
	args := []any{Now()}
	if p.Type != nil {
		if *p.Type != "additional" && *p.Type != "bulk" {
			return Token{}, validation("type", "invalid_type", "Jenis token tidak dikenal.")
		}
		set = append(set, "type = ?")
		args = append(args, *p.Type)
	}
	if p.Denomination != nil {
		set = append(set, "denomination = ?")
		args = append(args, *p.Denomination)
	}
	if p.Notes != nil {
		set = append(set, "notes = ?")
		args = append(args, *p.Notes)
	}
	if p.PurchasedAt != nil {
		ts, err := ParseTime(*p.PurchasedAt)
		if err != nil {
			return Token{}, validation("purchasedAt", "invalid_timestamp", "Waktu pembelian tidak valid.")
		}
		set = append(set, "purchased_at = ?")
		args = append(args, FormatTime(ts))
	}
	args = append(args, id)
	if _, err := db.SQL.Exec(`update tokens set `+strings.Join(set, ", ")+` where id = ?`, args...); err != nil {
		return Token{}, err
	}
	return db.GetToken(id)
}

func (db *DB) DeleteToken(id string) error {
	t, err := db.GetToken(id)
	if err != nil {
		return err
	}
	if !t.CanDelete {
		return ErrTokenNotDeletable
	}
	_, err = db.SQL.Exec(`delete from tokens where id = ?`, id)
	return err
}

// MarkTokenUsed records the reading taken right after the code was entered and
// links it to the token (FR-13 step 2).
func (db *DB) MarkTokenUsed(id string, in ReadingInput) (Token, Reading, error) {
	t, err := db.GetToken(id)
	if err != nil {
		return Token{}, Reading{}, err
	}
	if t.Status != "unused" {
		return Token{}, Reading{}, validation("id", "token_not_unused", "Token tidak berstatus belum dipakai.")
	}
	reading := in
	reading.ID = ""
	reading.MeterName = "main"
	reading.Source = "token"
	reading.TokenID = &id
	reading.RechargeAmount = &t.Kwh
	reading.Cost = t.Cost
	row, err := db.UpsertReading(reading)
	if err != nil {
		return Token{}, Reading{}, err
	}
	now := Now()
	_, err = db.SQL.Exec(`update tokens set status = 'used', linked_reading_id = ?, updated_at = ? where id = ?`, row.ID, now, id)
	if err != nil {
		// ponytail: best-effort rollback of the reading; upgrade to one shared
		// transaction if a DB error here ever actually happens.
		_ = db.SQL.QueryRow(`delete from readings where id = ?`, row.ID)
		return Token{}, Reading{}, err
	}
	t, err = db.GetToken(id)
	return t, row, err
}

// CancelToken undoes a not-locked use, removing the linked reading. It is
// idempotent on an unused token.
func (db *DB) CancelToken(id string) (Token, *string, error) {
	t, err := db.GetToken(id)
	if err != nil {
		return Token{}, nil, err
	}
	switch t.Status {
	case "locked":
		return Token{}, nil, ErrTokenLocked
	case "unused":
		return t, nil, nil
	}
	var cancelled *string
	if t.LinkedReadingID != nil {
		rid := *t.LinkedReadingID
		if err := db.DeleteReading(rid); err != nil && err != ErrNotFound {
			return Token{}, nil, err
		}
		cancelled = &rid
	}
	_, err = db.SQL.Exec(`update tokens set status = 'unused', linked_reading_id = null, locked_at = null, updated_at = ? where id = ?`, Now(), id)
	if err != nil {
		return Token{}, nil, err
	}
	t, err = db.GetToken(id)
	return t, cancelled, err
}

// RevealTokenCode decrypts the stored code. Callers must log the access and
// send Cache-Control: no-store.
func (db *DB) RevealTokenCode(id string) (string, error) {
	var enc []byte
	err := db.SQL.QueryRow(`select code_enc from tokens where id = ?`, id).Scan(&enc)
	if isNoRows(err) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if len(enc) == 0 {
		return "", ErrNotFound
	}
	return db.decrypt(enc)
}

func (db *DB) denominations() ([]int64, error) {
	var raw string
	err := db.SQL.QueryRow(`select denominations from settings where id = 1`).Scan(&raw)
	if isNoRows(err) {
		return DefaultDenominations, nil
	}
	if err != nil {
		return nil, err
	}
	var list []int64
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return DefaultDenominations, nil
	}
	return list, nil
}

func (db *DB) gcm() (cipher.AEAD, error) {
	key := db.Secret
	if len(key) == 0 {
		key = []byte("meteran-development-key-not-for-production")
	}
	if len(key) != 32 {
		sum := make([]byte, 32)
		copy(sum, key)
		key = sum
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (db *DB) encrypt(plain string) ([]byte, error) {
	aead, err := db.gcm()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

func (db *DB) decrypt(enc []byte) (string, error) {
	aead, err := db.gcm()
	if err != nil {
		return "", err
	}
	if len(enc) < aead.NonceSize() {
		return "", sql.ErrNoRows
	}
	plain, err := aead.Open(nil, enc[:aead.NonceSize()], enc[aead.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}