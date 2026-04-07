package store

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

type DB struct{ db *sql.DB }
type Watch struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Name        string `json:"name,omitempty"`
	WebhookURL  string `json:"webhook_url,omitempty"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	ChangeCount int    `json:"change_count"`
	LastChange  string `json:"last_change,omitempty"`
}
type Change struct {
	ID        string `json:"id"`
	WatchID   string `json:"watch_id"`
	FileName  string `json:"file_name"`
	Action    string `json:"action"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

func Open(d string) (*DB, error) {
	if err := os.MkdirAll(d, 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(d, "watcher.db")+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS watches(id TEXT PRIMARY KEY,path TEXT NOT NULL,name TEXT DEFAULT '',webhook_url TEXT DEFAULT '',enabled INTEGER DEFAULT 1,created_at TEXT DEFAULT(datetime('now')))`,
		`CREATE TABLE IF NOT EXISTS changes(id TEXT PRIMARY KEY,watch_id TEXT NOT NULL,file_name TEXT DEFAULT '',action TEXT DEFAULT 'modified',size INTEGER DEFAULT 0,created_at TEXT DEFAULT(datetime('now')))`,
		`CREATE INDEX IF NOT EXISTS idx_changes_watch ON changes(watch_id)`,
	} {
		if _, err := db.Exec(q); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS extras(resource TEXT NOT NULL,record_id TEXT NOT NULL,data TEXT NOT NULL DEFAULT '{}',PRIMARY KEY(resource, record_id))`)
	return &DB{db: db}, nil
}
func (d *DB) Close() error { return d.db.Close() }
func genID() string        { return fmt.Sprintf("%d", time.Now().UnixNano()) }
func now() string          { return time.Now().UTC().Format(time.RFC3339) }
func (d *DB) CreateWatch(w *Watch) error {
	w.ID = genID()
	w.CreatedAt = now()
	en := 1
	if !w.Enabled {
		en = 0
	}
	_, err := d.db.Exec(`INSERT INTO watches(id,path,name,webhook_url,enabled,created_at)VALUES(?,?,?,?,?,?)`, w.ID, w.Path, w.Name, w.WebhookURL, en, w.CreatedAt)
	return err
}
func (d *DB) GetWatch(id string) *Watch {
	var w Watch
	var en int
	if d.db.QueryRow(`SELECT id,path,name,webhook_url,enabled,created_at FROM watches WHERE id=?`, id).Scan(&w.ID, &w.Path, &w.Name, &w.WebhookURL, &en, &w.CreatedAt) != nil {
		return nil
	}
	w.Enabled = en == 1
	d.db.QueryRow(`SELECT COUNT(*) FROM changes WHERE watch_id=?`, w.ID).Scan(&w.ChangeCount)
	d.db.QueryRow(`SELECT created_at FROM changes WHERE watch_id=? ORDER BY created_at DESC LIMIT 1`, w.ID).Scan(&w.LastChange)
	return &w
}
func (d *DB) ListWatches() []Watch {
	rows, _ := d.db.Query(`SELECT id,path,name,webhook_url,enabled,created_at FROM watches ORDER BY name,path`)
	if rows == nil {
		return nil
	}
	defer rows.Close()
	var o []Watch
	for rows.Next() {
		var w Watch
		var en int
		rows.Scan(&w.ID, &w.Path, &w.Name, &w.WebhookURL, &en, &w.CreatedAt)
		w.Enabled = en == 1
		d.db.QueryRow(`SELECT COUNT(*) FROM changes WHERE watch_id=?`, w.ID).Scan(&w.ChangeCount)
		d.db.QueryRow(`SELECT created_at FROM changes WHERE watch_id=? ORDER BY created_at DESC LIMIT 1`, w.ID).Scan(&w.LastChange)
		o = append(o, w)
	}
	return o
}
func (d *DB) DeleteWatch(id string) error {
	d.db.Exec(`DELETE FROM changes WHERE watch_id=?`, id)
	_, err := d.db.Exec(`DELETE FROM watches WHERE id=?`, id)
	return err
}
func (d *DB) ToggleWatch(id string) error {
	_, err := d.db.Exec(`UPDATE watches SET enabled=1-enabled WHERE id=?`, id)
	return err
}
func (d *DB) RecordChange(c *Change) error {
	c.ID = genID()
	c.CreatedAt = now()
	_, err := d.db.Exec(`INSERT INTO changes VALUES(?,?,?,?,?,?)`, c.ID, c.WatchID, c.FileName, c.Action, c.Size, c.CreatedAt)
	return err
}
func (d *DB) ListChanges(watchID string, limit int) []Change {
	if limit <= 0 {
		limit = 50
	}
	rows, _ := d.db.Query(`SELECT id,watch_id,file_name,action,size,created_at FROM changes WHERE watch_id=? ORDER BY created_at DESC LIMIT ?`, watchID, limit)
	if rows == nil {
		return nil
	}
	defer rows.Close()
	var o []Change
	for rows.Next() {
		var c Change
		rows.Scan(&c.ID, &c.WatchID, &c.FileName, &c.Action, &c.Size, &c.CreatedAt)
		o = append(o, c)
	}
	return o
}

type Stats struct {
	Watches int `json:"watches"`
	Changes int `json:"changes"`
}

func (d *DB) Stats() Stats {
	var s Stats
	d.db.QueryRow(`SELECT COUNT(*) FROM watches`).Scan(&s.Watches)
	d.db.QueryRow(`SELECT COUNT(*) FROM changes`).Scan(&s.Changes)
	return s
}

// ─── Extras: generic key-value storage for personalization custom fields ───

func (d *DB) GetExtras(resource, recordID string) string {
	var data string
	err := d.db.QueryRow(
		`SELECT data FROM extras WHERE resource=? AND record_id=?`,
		resource, recordID,
	).Scan(&data)
	if err != nil || data == "" {
		return "{}"
	}
	return data
}

func (d *DB) SetExtras(resource, recordID, data string) error {
	if data == "" {
		data = "{}"
	}
	_, err := d.db.Exec(
		`INSERT INTO extras(resource, record_id, data) VALUES(?, ?, ?)
		 ON CONFLICT(resource, record_id) DO UPDATE SET data=excluded.data`,
		resource, recordID, data,
	)
	return err
}

func (d *DB) DeleteExtras(resource, recordID string) error {
	_, err := d.db.Exec(
		`DELETE FROM extras WHERE resource=? AND record_id=?`,
		resource, recordID,
	)
	return err
}

func (d *DB) AllExtras(resource string) map[string]string {
	out := make(map[string]string)
	rows, _ := d.db.Query(
		`SELECT record_id, data FROM extras WHERE resource=?`,
		resource,
	)
	if rows == nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, data string
		rows.Scan(&id, &data)
		out[id] = data
	}
	return out
}
