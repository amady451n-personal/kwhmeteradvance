package store

// PushKeys are the browser's subscription keys.
type PushKeys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}

// PushSubscription is a stored Web Push endpoint.
type PushSubscription struct {
	ID        string   `json:"id"`
	Endpoint  string   `json:"endpoint"`
	Keys      PushKeys `json:"keys"`
	CreatedAt string   `json:"createdAt"`
}

// Subscribe upserts by endpoint: a browser re-subscribing keeps its row id.
func (db *DB) Subscribe(endpoint, p256dh, authKey string) (PushSubscription, error) {
	id := NewID("push")
	_, err := db.SQL.Exec(`insert into push_subscriptions (id, endpoint, p256dh, auth, created_at)
		values (?, ?, ?, ?, ?)
		on conflict (endpoint) do update set p256dh = excluded.p256dh, auth = excluded.auth`,
		id, endpoint, p256dh, authKey, Now())
	if err != nil {
		return PushSubscription{}, err
	}
	return db.getSubscriptionByEndpoint(endpoint)
}

func (db *DB) getSubscriptionByEndpoint(endpoint string) (PushSubscription, error) {
	var s PushSubscription
	err := db.SQL.QueryRow(`select id, endpoint, p256dh, auth, created_at from push_subscriptions where endpoint = ?`,
		endpoint).Scan(&s.ID, &s.Endpoint, &s.Keys.P256dh, &s.Keys.Auth, &s.CreatedAt)
	if isNoRows(err) {
		return PushSubscription{}, ErrNotFound
	}
	return s, err
}

func (db *DB) Unsubscribe(id string) error {
	res, err := db.SQL.Exec(`delete from push_subscriptions where id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (db *DB) CountSubscriptions() (int, error) {
	var n int
	err := db.SQL.QueryRow(`select count(*) from push_subscriptions`).Scan(&n)
	return n, err
}