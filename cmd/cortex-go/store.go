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
	ID        int64  `json:"id"`
	FeedID    string `json:"feedId"`
	Author    string `json:"author"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
}

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
	const schema = `CREATE TABLE IF NOT EXISTS synthetic_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		feed_id TEXT NOT NULL CHECK (feed_id IN ('a', 'b')),
		author TEXT NOT NULL,
		message TEXT NOT NULL,
		created_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_synthetic_events_created ON synthetic_events(id DESC);`
	if _, err = db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("preparar tabla de eventos sintéticos: %w", err)
	}
	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("comprobar SQLite: %w", err)
	}
	return &eventStore{db: db, path: path}, nil
}

func (s *eventStore) Close() error { return s.db.Close() }

func (s *eventStore) Add(ctx context.Context, event syntheticEvent) (syntheticEvent, error) {
	event.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO synthetic_events(feed_id, author, message, created_at) VALUES(?, ?, ?, ?)`,
		event.FeedID, event.Author, event.Message, event.CreatedAt)
	if err != nil {
		return syntheticEvent{}, err
	}
	event.ID, err = result.LastInsertId()
	return event, err
}

func (s *eventStore) Recent(ctx context.Context, limit int) ([]syntheticEvent, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, feed_id, author, message, created_at FROM synthetic_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]syntheticEvent, 0)
	for rows.Next() {
		var event syntheticEvent
		if err := rows.Scan(&event.ID, &event.FeedID, &event.Author, &event.Message, &event.CreatedAt); err != nil {
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
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM synthetic_events WHERE feed_id = ?`, feedID).Scan(&count)
	return count, err
}

func (s *eventStore) Total(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM synthetic_events`).Scan(&count)
	return count, err
}

func (s *eventStore) Latest(ctx context.Context, feedID string) (string, error) {
	var timestamp string
	err := s.db.QueryRowContext(ctx,
		`SELECT created_at FROM synthetic_events WHERE feed_id = ? ORDER BY id DESC LIMIT 1`, feedID).Scan(&timestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return timestamp, err
}

func (s *eventStore) LatestEventTime(ctx context.Context) (string, error) {
	var timestamp string
	err := s.db.QueryRowContext(ctx, `SELECT created_at FROM synthetic_events ORDER BY id DESC LIMIT 1`).Scan(&timestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return timestamp, err
}
