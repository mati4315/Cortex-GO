package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type syntheticEvent struct {
	ID         int64  `json:"id"`
	FeedID     string `json:"feedId"`
	SourceID   string `json:"sourceId,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
	StreamID   string `json:"streamId,omitempty"`
	Author     string `json:"author"`
	Message    string `json:"message"`
	EventAt    string `json:"eventAt,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

type capturedEvent struct {
	FeedID     string
	SourceID   string
	ExternalID string
	StreamID   string
	Author     string
	Message    string
	EventAt    string
	CapturedAt string
	DedupKey   string
}

const currentSchemaVersion = 1

type eventStore struct {
	db   *sql.DB
	path string
}

func openEventStore(path string) (*eventStore, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolver ruta de SQLite: %w", err)
	}
	path = filepath.Clean(absolutePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("preparar carpeta de datos: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("abrir SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=5000", "PRAGMA foreign_keys=ON"} {
		if _, err = db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("configurar SQLite: %w", err)
		}
	}
	if err = migrateEventSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("preparar esquema de eventos: %w", err)
	}
	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("comprobar SQLite: %w", err)
	}
	return &eventStore{db: db, path: path}, nil
}

func migrateEventSchema(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return err
	}
	var version int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version > currentSchemaVersion {
		return fmt.Errorf("la base usa una versión de esquema más nueva (%d)", version)
	}
	if version < 1 {
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id TEXT NOT NULL,
			feed_id TEXT NOT NULL,
			external_id TEXT NOT NULL DEFAULT '',
			stream_id TEXT NOT NULL DEFAULT '',
			author TEXT NOT NULL,
			message TEXT NOT NULL,
			event_at TEXT NOT NULL,
			captured_at TEXT NOT NULL,
			dedup_key TEXT
		)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_events_captured ON events(id DESC);
			CREATE INDEX IF NOT EXISTS idx_events_source_feed ON events(source_id, feed_id, id DESC);
			CREATE UNIQUE INDEX IF NOT EXISTS idx_events_deduplicate ON events(source_id, dedup_key) WHERE dedup_key IS NOT NULL`); err != nil {
			return err
		}
		var legacyTableExists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='synthetic_events')`).Scan(&legacyTableExists); err != nil {
			return err
		}
		if legacyTableExists {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO events
				(id, source_id, feed_id, external_id, stream_id, author, message, event_at, captured_at, dedup_key)
				SELECT id, 'test:' || feed_id, feed_id, 'legacy-' || id, '', author, message, created_at, created_at, 'legacy-' || id
				FROM synthetic_events`); err != nil {
				return fmt.Errorf("migrar eventos de prueba existentes: %w", err)
			}
			if _, err := tx.Exec(`DROP TABLE synthetic_events`); err != nil {
				return fmt.Errorf("retirar la tabla anterior después de migrarla: %w", err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(1, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *eventStore) Close() error { return s.db.Close() }

func (s *eventStore) Add(ctx context.Context, event syntheticEvent) (syntheticEvent, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stored, _, err := s.AddCaptured(ctx, capturedEvent{
		FeedID: event.FeedID, SourceID: "test:" + event.FeedID,
		Author: event.Author, Message: event.Message, EventAt: now, CapturedAt: now,
	})
	return stored, err
}

func (s *eventStore) AddCaptured(ctx context.Context, event capturedEvent) (syntheticEvent, bool, error) {
	if event.CapturedAt == "" {
		event.CapturedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if event.EventAt == "" {
		event.EventAt = event.CapturedAt
	}
	var dedupKey any
	if event.DedupKey != "" {
		dedupKey = event.DedupKey
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO events
		(source_id, feed_id, external_id, stream_id, author, message, event_at, captured_at, dedup_key)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(source_id, dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING`,
		event.SourceID, event.FeedID, event.ExternalID, event.StreamID, event.Author, event.Message, event.EventAt, event.CapturedAt, dedupKey)
	if err != nil {
		return syntheticEvent{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return syntheticEvent{}, false, err
	}
	if affected == 0 {
		stored, err := s.findByDedupKey(ctx, event.SourceID, event.DedupKey)
		return stored, false, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return syntheticEvent{}, false, err
	}
	return syntheticEvent{
		ID: id, FeedID: event.FeedID, SourceID: event.SourceID, ExternalID: event.ExternalID,
		StreamID: event.StreamID, Author: event.Author, Message: event.Message,
		EventAt: event.EventAt, CreatedAt: event.CapturedAt,
	}, true, nil
}

func (s *eventStore) findByDedupKey(ctx context.Context, sourceID, dedupKey string) (syntheticEvent, error) {
	var event syntheticEvent
	err := s.db.QueryRowContext(ctx, `SELECT id, feed_id, source_id, external_id, stream_id, author, message, event_at, captured_at
		FROM events WHERE source_id = ? AND dedup_key = ?`, sourceID, dedupKey).Scan(
		&event.ID, &event.FeedID, &event.SourceID, &event.ExternalID, &event.StreamID,
		&event.Author, &event.Message, &event.EventAt, &event.CreatedAt)
	return event, err
}

func (s *eventStore) Recent(ctx context.Context, limit int) ([]syntheticEvent, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, feed_id, source_id, external_id, stream_id, author, message, event_at, captured_at FROM events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]syntheticEvent, 0)
	for rows.Next() {
		var event syntheticEvent
		if err := rows.Scan(&event.ID, &event.FeedID, &event.SourceID, &event.ExternalID, &event.StreamID,
			&event.Author, &event.Message, &event.EventAt, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// El panel muestra primero el comentario más antiguo dentro de la ventana.
	for left, right := 0, len(events)-1; left < right; left, right = left+1, right-1 {
		events[left], events[right] = events[right], events[left]
	}
	return events, nil
}

func (s *eventStore) Count(ctx context.Context, feedID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE feed_id = ?`, feedID).Scan(&count)
	return count, err
}

func (s *eventStore) Delete(ctx context.Context, id int64) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *eventStore) DeleteAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM events`)
	return err
}

func (s *eventStore) SchemaVersion(ctx context.Context) (int, error) {
	var version int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	return version, err
}

func (s *eventStore) Total(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&count)
	return count, err
}

func (s *eventStore) Latest(ctx context.Context, feedID string) (string, error) {
	var timestamp string
	err := s.db.QueryRowContext(ctx,
		`SELECT captured_at FROM events WHERE feed_id = ? ORDER BY id DESC LIMIT 1`, feedID).Scan(&timestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return timestamp, err
}

func (s *eventStore) LatestEventTime(ctx context.Context) (string, error) {
	var timestamp string
	err := s.db.QueryRowContext(ctx, `SELECT captured_at FROM events ORDER BY id DESC LIMIT 1`).Scan(&timestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return timestamp, err
}
