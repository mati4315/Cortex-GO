package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type syntheticEvent struct {
	ID           int64             `json:"id"`
	FeedID       string            `json:"feedId"`
	SourceID     string            `json:"sourceId,omitempty"`
	ExternalID   string            `json:"externalId,omitempty"`
	StreamID     string            `json:"streamId,omitempty"`
	Author       string            `json:"author"`
	Message      string            `json:"message"`
	EventAt      string            `json:"eventAt,omitempty"`
	CreatedAt    string            `json:"createdAt"`
	RuloResponse *ruloPresentation `json:"ruloResponse,omitempty"`
}

type capturedEvent struct {
	FeedID        string
	SourceID      string
	ExternalID    string
	StreamID      string
	ParticipantID string
	Author        string
	Message       string
	EventAt       string
	CapturedAt    string
	DedupKey      string
}

type dispatchEvent struct {
	Sequence         int64          `json:"sequence"`
	EventID          int64          `json:"eventId"`
	ProgramSessionID string         `json:"programSessionId,omitempty"`
	ParticipantID    string         `json:"participantId,omitempty"`
	Type             string         `json:"type"`
	Source           string         `json:"source"`
	Scope            string         `json:"scope"`
	Occurred         string         `json:"occurredAt"`
	Received         string         `json:"receivedAt"`
	Data             syntheticEvent `json:"data"`
}

const currentSchemaVersion = 18

var (
	errProgramSessionActive = errors.New("ya hay un programa activo")
	errNoProgramSession     = errors.New("no hay un programa activo")
)

type programSession struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt,omitempty"`
	Status    string `json:"status"`
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
		version = 1
	}
	if version < 2 {
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS module_settings (
				module_id TEXT PRIMARY KEY,
				settings_json TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
			`CREATE TABLE IF NOT EXISTS module_runtime (
				module_id TEXT PRIMARY KEY,
				runtime_json TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
			`CREATE TABLE IF NOT EXISTS module_assets (
				asset_id TEXT PRIMARY KEY,
				content_type TEXT NOT NULL,
				original_name TEXT NOT NULL,
				byte_size INTEGER NOT NULL,
				content BLOB NOT NULL,
				uploaded_at TEXT NOT NULL
			)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return fmt.Errorf("crear almacenamiento de módulos: %w", err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(2, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		version = 2
	}
	if version < 3 {
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS event_archives (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			program TEXT NOT NULL UNIQUE,
			events_json TEXT NOT NULL,
			event_count INTEGER NOT NULL,
			created_at TEXT NOT NULL
		)`); err != nil {
			return fmt.Errorf("crear archivo de chats Facebook: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(3, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		version = 3
	}
	if version < 4 {
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_events_event_at_id ON events(event_at, id DESC);
			CREATE INDEX IF NOT EXISTS idx_events_stream_id ON events(stream_id, id DESC)`); err != nil {
			return fmt.Errorf("crear índices para el historial: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(4, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if version < 5 {
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS event_dispatch (sequence INTEGER PRIMARY KEY AUTOINCREMENT, event_id INTEGER NOT NULL, event_json TEXT NOT NULL, received_at TEXT NOT NULL)`,
			`CREATE INDEX IF NOT EXISTS idx_event_dispatch_event ON event_dispatch(event_id)`,
			`CREATE TABLE IF NOT EXISTS dispatch_cursors (consumer_id TEXT PRIMARY KEY, sequence INTEGER NOT NULL DEFAULT 0, updated_at TEXT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS rulo_settings (setting_id TEXT PRIMARY KEY, value_json TEXT NOT NULL, updated_at TEXT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS rulo_rules (id INTEGER PRIMARY KEY AUTOINCREMENT, category TEXT NOT NULL, scope TEXT NOT NULL, instruction TEXT NOT NULL, active INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS rulo_lab_sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, input_text TEXT NOT NULL, classification TEXT NOT NULL, response_text TEXT NOT NULL, created_at TEXT NOT NULL)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return fmt.Errorf("crear almacenamiento inicial de Rulo: %w", err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(5, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if version < 6 {
		if _, err := tx.Exec(`ALTER TABLE events ADD COLUMN participant_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("agregar la identidad opcional del participante a eventos: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(6, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if version < 7 {
		if _, err := tx.Exec(`ALTER TABLE rulo_lab_sessions ADD COLUMN response_to INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("agregar la relación de aclaraciones del laboratorio: %w", err)
		}
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS rulo_clarifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			intent TEXT NOT NULL,
			prompt TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			answered_at TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(session_id) REFERENCES rulo_lab_sessions(id) ON DELETE CASCADE
		)`); err != nil {
			return fmt.Errorf("crear aclaraciones del laboratorio: %w", err)
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_rulo_clarifications_pending ON rulo_clarifications(status, expires_at)`); err != nil {
			return fmt.Errorf("indexar vencimiento de aclaraciones: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(7, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if version < 8 {
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS rulo_responses (
				response_id TEXT PRIMARY KEY,
				event_id INTEGER NOT NULL UNIQUE,
				signal TEXT NOT NULL,
				text TEXT NOT NULL,
				version INTEGER NOT NULL DEFAULT 1,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				FOREIGN KEY(event_id) REFERENCES events(id) ON DELETE CASCADE
			)`,
			`CREATE TABLE IF NOT EXISTS rulo_response_updates (
				sequence INTEGER PRIMARY KEY AUTOINCREMENT,
				response_id TEXT NOT NULL,
				version INTEGER NOT NULL,
				signal TEXT NOT NULL,
				text TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				UNIQUE(response_id, version),
				FOREIGN KEY(response_id) REFERENCES rulo_responses(response_id) ON DELETE CASCADE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_rulo_response_updates_sequence ON rulo_response_updates(sequence)`,
			`CREATE INDEX IF NOT EXISTS idx_rulo_responses_updated ON rulo_responses(updated_at DESC, event_id DESC)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return fmt.Errorf("crear salida persistida de Rulo: %w", err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(8, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if version < 9 {
		if _, err := tx.Exec(`ALTER TABLE events ADD COLUMN program_session_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("agregar la sesión de programa a los eventos: %w", err)
		}
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS program_sessions (
			id TEXT PRIMARY KEY,
			label TEXT NOT NULL,
			started_at TEXT NOT NULL,
			ended_at TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL CHECK(status IN ('active','closed'))
		)`); err != nil {
			return fmt.Errorf("crear sesiones de programa: %w", err)
		}
		if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_program_sessions_active ON program_sessions(status) WHERE status='active'`); err != nil {
			return fmt.Errorf("limitar a un programa activo: %w", err)
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_events_program_session ON events(program_session_id, id DESC)`); err != nil {
			return fmt.Errorf("indexar eventos por programa: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(9, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if version < 10 {
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS spotify_operations (
				operation_id TEXT PRIMARY KEY,
				event_id INTEGER NOT NULL,
				program_session_id TEXT NOT NULL,
				track_uri TEXT NOT NULL,
				status TEXT NOT NULL CHECK(status IN ('pendiente','aceptado','completado','rechazado','fallido','incierto','vencido','cancelado')),
				detail TEXT NOT NULL DEFAULT '',
				sent_at TEXT NOT NULL DEFAULT '',
				retry_at TEXT NOT NULL DEFAULT '',
				accepted_at TEXT NOT NULL DEFAULT '',
				attempts INTEGER NOT NULL DEFAULT 0,
				expires_at TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_spotify_operations_pending ON spotify_operations(status, retry_at, expires_at)`,
			`CREATE INDEX IF NOT EXISTS idx_spotify_operations_session ON spotify_operations(program_session_id, created_at DESC)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return fmt.Errorf("crear registro durable de pedidos Spotify: %w", err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(10, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if version < 11 {
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS spotify_operation_order (
			queue_sequence INTEGER PRIMARY KEY AUTOINCREMENT,
			operation_id TEXT NOT NULL UNIQUE
		)`); err != nil {
			return fmt.Errorf("crear orden durable de pedidos Spotify: %w", err)
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO spotify_operation_order(operation_id)
			SELECT operation_id FROM spotify_operations ORDER BY created_at, operation_id`); err != nil {
			return fmt.Errorf("migrar el orden previo de pedidos Spotify: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(11, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if version < 12 {
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS rulo_eval_cases (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			input_text TEXT NOT NULL,
			expected_intents_json TEXT NOT NULL,
			expected_outcome TEXT NOT NULL CHECK(expected_outcome IN ('response','clarification','silence')),
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`); err != nil {
			return fmt.Errorf("crear casos de evaluación del laboratorio Rulo: %w", err)
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		seeds := []struct {
			name, input, intentsJSON, outcome string
		}{
			{"Pregunta conversacional", "Rulo, ¿cómo estás?", `["question"]`, "response"},
			{"Pedido musical claro", "Agregá «Persiana Americana» a la cola", `["music_request"]`, "response"},
			{"Pedido musical ambiguo", "Agregá una canción a la cola", `["music_request"]`, "clarification"},
			{"Elección de número", "Para la lotería elijo el 24", `["lottery_request"]`, "response"},
			{"Lotería sin número", "¿Me anotás para la lotería?", `["lottery_request","question"]`, "clarification"},
			{"Comentario que puede quedar en silencio", "👌", `[]`, "silence"},
		}
		for _, seed := range seeds {
			if _, err := tx.Exec(`INSERT INTO rulo_eval_cases(name, input_text, expected_intents_json, expected_outcome, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)`, seed.name, seed.input, seed.intentsJSON, seed.outcome, now, now); err != nil {
				return fmt.Errorf("cargar ejemplos sintéticos del laboratorio Rulo: %w", err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(12, ?)`, now); err != nil {
			return err
		}
	}
	if version < 13 {
		if _, err := tx.Exec(`ALTER TABLE rulo_rules ADD COLUMN example_text TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("agregar ejemplos guardados a las reglas de Rulo: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(13, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if version < 14 {
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS rulo_ai_request_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_kind TEXT NOT NULL CHECK(request_kind IN ('simulation','rule_proposal','rule_test')),
			requested_at INTEGER NOT NULL
		)`); err != nil {
			return fmt.Errorf("crear el límite de uso IA del laboratorio Rulo: %w", err)
		}
		if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_rulo_ai_request_usage_requested_at ON rulo_ai_request_usage(requested_at)`); err != nil {
			return fmt.Errorf("indexar el uso IA del laboratorio Rulo: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(14, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if version < 15 {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := tx.Exec(`INSERT INTO rulo_eval_cases(name, input_text, expected_intents_json, expected_outcome, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?, ?)`, "Consulta estadística del historial", "Rulo, ¿cuántos comentarios hubo en el historial?", `["history_stats_request","question"]`, "response", now, now); err != nil {
			return fmt.Errorf("agregar el caso sintético de estadísticas de Rulo: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(15, ?)`, now); err != nil {
			return err
		}
	}
	if version < 16 {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS rulo_live_ai_usage (
				event_id INTEGER PRIMARY KEY,
				program_session_id TEXT NOT NULL,
				requested_at INTEGER NOT NULL,
				status TEXT NOT NULL CHECK(status IN ('reserved','answered','silent','failed','uncertain','rate_limited')),
				FOREIGN KEY(event_id) REFERENCES events(id) ON DELETE CASCADE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_rulo_live_ai_usage_requested_at ON rulo_live_ai_usage(requested_at, status)`,
			`CREATE INDEX IF NOT EXISTS idx_rulo_live_ai_usage_session ON rulo_live_ai_usage(program_session_id, requested_at, status)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return fmt.Errorf("preparar conversación en vivo acotada de Rulo: %w", err)
			}
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO rulo_settings(setting_id, value_json, updated_at) VALUES('live_conversation_enabled', 'false', ?)`, now); err != nil {
			return fmt.Errorf("desactivar la conversación en vivo por defecto: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(16, ?)`, now); err != nil {
			return err
		}
	}
	if version < 17 {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := tx.Exec(`ALTER TABLE rulo_live_ai_usage ADD COLUMN token_units INTEGER NOT NULL DEFAULT 0 CHECK(token_units >= 0)`); err != nil {
			return fmt.Errorf("agregar el registro de presupuesto de tokens de Rulo: %w", err)
		}
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS rulo_live_ai_eval_usage (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				requested_at INTEGER NOT NULL,
				status TEXT NOT NULL CHECK(status IN ('reserved','answered','silent','failed','uncertain')),
				token_units INTEGER NOT NULL CHECK(token_units >= 0)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_rulo_live_ai_eval_usage_requested_at ON rulo_live_ai_eval_usage(requested_at, status)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return fmt.Errorf("preparar presupuesto y evaluación aislada de Rulo: %w", err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(17, ?)`, now); err != nil {
			return err
		}
	}
	if version < 18 {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		for _, statement := range []string{
			`ALTER TABLE rulo_live_ai_usage ADD COLUMN budget_usage_id INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE rulo_live_ai_eval_usage ADD COLUMN budget_usage_id INTEGER NOT NULL DEFAULT 0`,
			`CREATE TABLE IF NOT EXISTS rulo_live_ai_budget_usage (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				request_kind TEXT NOT NULL CHECK(request_kind IN ('live','evaluation')),
				program_session_id TEXT NOT NULL DEFAULT '',
				requested_at INTEGER NOT NULL,
				status TEXT NOT NULL CHECK(status IN ('reserved','answered','silent','failed','uncertain','rate_limited')),
				token_units INTEGER NOT NULL CHECK(token_units >= 0)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_rulo_live_ai_budget_usage_requested_at ON rulo_live_ai_budget_usage(requested_at, status)`,
			`CREATE INDEX IF NOT EXISTS idx_rulo_live_ai_budget_usage_session ON rulo_live_ai_budget_usage(request_kind, program_session_id, requested_at)`,
		} {
			if _, err := tx.Exec(statement); err != nil {
				return fmt.Errorf("crear ledger de presupuesto IA independiente del historial: %w", err)
			}
		}
		liveRows, err := tx.Query(`SELECT event_id, program_session_id, requested_at, status, token_units FROM rulo_live_ai_usage ORDER BY event_id`)
		if err != nil {
			return err
		}
		type liveUsageRow struct {
			eventID    int64
			sessionID  string
			requested  int64
			status     string
			tokenUnits int64
		}
		liveUsageRows := make([]liveUsageRow, 0)
		for liveRows.Next() {
			var row liveUsageRow
			if err := liveRows.Scan(&row.eventID, &row.sessionID, &row.requested, &row.status, &row.tokenUnits); err != nil {
				liveRows.Close()
				return err
			}
			liveUsageRows = append(liveUsageRows, row)
		}
		if err := liveRows.Err(); err != nil {
			liveRows.Close()
			return err
		}
		liveRows.Close()
		for _, row := range liveUsageRows {
			result, err := tx.Exec(`INSERT INTO rulo_live_ai_budget_usage(request_kind, program_session_id, requested_at, status, token_units) VALUES('live', ?, ?, ?, ?)`, row.sessionID, row.requested, row.status, row.tokenUnits)
			if err != nil {
				return err
			}
			budgetID, err := result.LastInsertId()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE rulo_live_ai_usage SET budget_usage_id=? WHERE event_id=?`, budgetID, row.eventID); err != nil {
				return err
			}
		}
		evalRows, err := tx.Query(`SELECT id, requested_at, status, token_units FROM rulo_live_ai_eval_usage ORDER BY id`)
		if err != nil {
			return err
		}
		type evalUsageRow struct {
			id        int64
			requested int64
			status    string
			tokens    int64
		}
		evalUsageRows := make([]evalUsageRow, 0)
		for evalRows.Next() {
			var row evalUsageRow
			if err := evalRows.Scan(&row.id, &row.requested, &row.status, &row.tokens); err != nil {
				evalRows.Close()
				return err
			}
			evalUsageRows = append(evalUsageRows, row)
		}
		if err := evalRows.Err(); err != nil {
			evalRows.Close()
			return err
		}
		evalRows.Close()
		for _, row := range evalUsageRows {
			result, err := tx.Exec(`INSERT INTO rulo_live_ai_budget_usage(request_kind, requested_at, status, token_units) VALUES('evaluation', ?, ?, ?)`, row.requested, row.status, row.tokens)
			if err != nil {
				return err
			}
			budgetID, err := result.LastInsertId()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE rulo_live_ai_eval_usage SET budget_usage_id=? WHERE id=?`, budgetID, row.id); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(18, ?)`, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *eventStore) Close() error { return s.db.Close() }

func (s *eventStore) ActiveProgramSession(ctx context.Context) (*programSession, error) {
	var session programSession
	err := s.db.QueryRowContext(ctx, `SELECT id, label, started_at, ended_at, status FROM program_sessions WHERE status='active' LIMIT 1`).Scan(
		&session.ID, &session.Label, &session.StartedAt, &session.EndedAt, &session.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *eventStore) StartProgramSession(ctx context.Context, label string) (programSession, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return programSession{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	session := programSession{ID: hex.EncodeToString(bytes), Label: label, StartedAt: now, Status: "active"}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return programSession{}, err
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM program_sessions WHERE status='active' LIMIT 1`).Scan(&exists)
	if err == nil {
		return programSession{}, errProgramSessionActive
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return programSession{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO program_sessions(id, label, started_at, status) VALUES(?, ?, ?, 'active')`, session.ID, session.Label, session.StartedAt); err != nil {
		return programSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return programSession{}, err
	}
	return session, nil
}

func (s *eventStore) EndProgramSession(ctx context.Context) (*programSession, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var session programSession
	err = tx.QueryRowContext(ctx, `SELECT id, label, started_at FROM program_sessions WHERE status='active' LIMIT 1`).Scan(
		&session.ID, &session.Label, &session.StartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNoProgramSession
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE program_sessions SET status='closed', ended_at=? WHERE id=? AND status='active'`, now, session.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	session.EndedAt = now
	session.Status = "closed"
	return &session, nil
}

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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return syntheticEvent{}, false, err
	}
	defer tx.Rollback()
	programSessionID := ""
	if !strings.HasPrefix(event.SourceID, "test:") {
		_ = tx.QueryRowContext(ctx, `SELECT id FROM program_sessions WHERE status='active' LIMIT 1`).Scan(&programSessionID)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO events
		(source_id, feed_id, external_id, stream_id, participant_id, program_session_id, author, message, event_at, captured_at, dedup_key)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(source_id, dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING`,
		event.SourceID, event.FeedID, event.ExternalID, event.StreamID, event.ParticipantID, programSessionID, event.Author, event.Message, event.EventAt, event.CapturedAt, dedupKey)
	if err != nil {
		return syntheticEvent{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return syntheticEvent{}, false, err
	}
	if affected == 0 {
		var stored syntheticEvent
		err := tx.QueryRowContext(ctx, `SELECT id, feed_id, source_id, external_id, stream_id, author, message, event_at, captured_at FROM events WHERE source_id = ? AND dedup_key = ?`, event.SourceID, event.DedupKey).Scan(
			&stored.ID, &stored.FeedID, &stored.SourceID, &stored.ExternalID, &stored.StreamID,
			&stored.Author, &stored.Message, &stored.EventAt, &stored.CreatedAt)
		return stored, false, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return syntheticEvent{}, false, err
	}
	stored := syntheticEvent{
		ID: id, FeedID: event.FeedID, SourceID: event.SourceID, ExternalID: event.ExternalID,
		StreamID: event.StreamID, Author: event.Author, Message: event.Message,
		EventAt: event.EventAt, CreatedAt: event.CapturedAt,
	}
	scope, eventType := "live", "facebook.comment"
	if strings.HasPrefix(event.SourceID, "test:") {
		scope, eventType = "lab", "lab.comment"
	}
	queued := dispatchEvent{EventID: id, ProgramSessionID: programSessionID, ParticipantID: event.ParticipantID, Type: eventType, Source: event.SourceID, Scope: scope, Occurred: event.EventAt, Received: event.CapturedAt, Data: stored}
	encoded, err := json.Marshal(queued)
	if err != nil {
		return syntheticEvent{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO event_dispatch(event_id, event_json, received_at) VALUES(?, ?, ?)`, id, string(encoded), event.CapturedAt); err != nil {
		return syntheticEvent{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return syntheticEvent{}, false, err
	}
	return stored, true, nil
}

func (s *eventStore) PendingDispatch(ctx context.Context, consumer string, limit int) ([]dispatchEvent, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	var cursor int64
	err := s.db.QueryRowContext(ctx, `SELECT sequence FROM dispatch_cursors WHERE consumer_id = ?`, consumer).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		cursor = 0
	} else if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT sequence, event_json FROM event_dispatch WHERE sequence > ? ORDER BY sequence LIMIT ?`, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]dispatchEvent, 0, limit)
	for rows.Next() {
		var item dispatchEvent
		var encoded string
		if err := rows.Scan(&item.Sequence, &encoded); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(encoded), &item); err != nil {
			return nil, fmt.Errorf("evento pendiente de despacho inválido: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *eventStore) RegisterDispatchConsumer(ctx context.Context, consumer string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM dispatch_cursors WHERE consumer_id = ?`, consumer).Scan(&exists); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var baseline int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM event_dispatch`).Scan(&baseline); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO dispatch_cursors(consumer_id, sequence, updated_at) VALUES(?, ?, ?)`, consumer, baseline, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *eventStore) AdvanceDispatchCursor(ctx context.Context, consumer string, sequence int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO dispatch_cursors(consumer_id, sequence, updated_at) VALUES(?, ?, ?)
		ON CONFLICT(consumer_id) DO UPDATE SET sequence = MAX(sequence, excluded.sequence), updated_at = excluded.updated_at`, consumer, sequence, now); err != nil {
		return err
	}
	var minimum int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MIN(sequence), 0) FROM dispatch_cursors`).Scan(&minimum); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM event_dispatch WHERE sequence <= ?`, minimum); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *eventStore) DispatchCursor(ctx context.Context, consumer string) (int64, error) {
	var cursor int64
	err := s.db.QueryRowContext(ctx, `SELECT sequence FROM dispatch_cursors WHERE consumer_id = ?`, consumer).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return cursor, err
}

func (s *eventStore) SaveRuloResponse(ctx context.Context, eventID int64, signal, text string) (ruloResponseUpdate, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ruloResponseUpdate{}, err
	}
	defer tx.Rollback()

	var response ruloPresentation
	var sourceID string
	if err := tx.QueryRowContext(ctx, `SELECT feed_id, source_id, stream_id FROM events WHERE id = ?`, eventID).Scan(&response.FeedID, &sourceID, &response.StreamID); err != nil {
		return ruloResponseUpdate{}, err
	}
	if strings.HasPrefix(sourceID, "test:") {
		return ruloResponseUpdate{}, errors.New("un evento de laboratorio no puede publicar una respuesta en vivo")
	}

	var createdAt, updatedAt, existingSignal, existingText string
	err = tx.QueryRowContext(ctx, `SELECT response_id, version, signal, text, created_at, updated_at FROM rulo_responses WHERE event_id = ?`, eventID).Scan(&response.ResponseID, &response.Version, &existingSignal, &existingText, &createdAt, &updatedAt)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if errors.Is(err, sql.ErrNoRows) {
		response.ResponseID = fmt.Sprintf("rulo-%d", eventID)
		response.Version = 1
		createdAt = now
		if _, err := tx.ExecContext(ctx, `INSERT INTO rulo_responses(response_id, event_id, signal, text, version, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?)`, response.ResponseID, eventID, signal, text, response.Version, createdAt, now); err != nil {
			return ruloResponseUpdate{}, err
		}
	} else if err != nil {
		return ruloResponseUpdate{}, err
	} else {
		if existingSignal == signal && existingText == text {
			response.EventID = eventID
			response.Signal = signal
			response.Text = text
			response.CreatedAt = createdAt
			response.UpdatedAt = updatedAt
			var sequence int64
			if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM rulo_response_updates WHERE response_id = ?`, response.ResponseID).Scan(&sequence); err != nil {
				return ruloResponseUpdate{}, err
			}
			if err := tx.Commit(); err != nil {
				return ruloResponseUpdate{}, err
			}
			return ruloResponseUpdate{Sequence: sequence, Response: response}, nil
		}
		response.Version++
		if _, err := tx.ExecContext(ctx, `UPDATE rulo_responses SET signal = ?, text = ?, version = ?, updated_at = ? WHERE event_id = ?`, signal, text, response.Version, now, eventID); err != nil {
			return ruloResponseUpdate{}, err
		}
	}
	response.EventID = eventID
	response.Signal = signal
	response.Text = text
	response.CreatedAt = createdAt
	response.UpdatedAt = now
	result, err := tx.ExecContext(ctx, `INSERT INTO rulo_response_updates(response_id, version, signal, text, updated_at) VALUES(?, ?, ?, ?, ?)`, response.ResponseID, response.Version, signal, text, now)
	if err != nil {
		return ruloResponseUpdate{}, err
	}
	sequence, err := result.LastInsertId()
	if err != nil {
		return ruloResponseUpdate{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM rulo_response_updates WHERE sequence <= (SELECT MAX(sequence) - 2000 FROM rulo_response_updates)`); err != nil {
		return ruloResponseUpdate{}, err
	}
	if err := tx.Commit(); err != nil {
		return ruloResponseUpdate{}, err
	}
	return ruloResponseUpdate{Sequence: sequence, Response: response, New: true}, nil
}

func (s *eventStore) RecentRuloResponses(ctx context.Context, limit int) ([]ruloPresentation, int64, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.response_id, r.event_id, r.version, r.signal, r.text, r.created_at, r.updated_at, e.feed_id, e.stream_id
		FROM rulo_responses r JOIN events e ON e.id = r.event_id ORDER BY r.updated_at DESC, r.event_id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	responses := make([]ruloPresentation, 0, limit)
	for rows.Next() {
		var response ruloPresentation
		if err := rows.Scan(&response.ResponseID, &response.EventID, &response.Version, &response.Signal, &response.Text, &response.CreatedAt, &response.UpdatedAt, &response.FeedID, &response.StreamID); err != nil {
			return nil, 0, err
		}
		responses = append(responses, response)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	var cursor int64
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM rulo_response_updates`).Scan(&cursor)
	return responses, cursor, err
}

func (s *eventStore) RuloResponseUpdates(ctx context.Context, afterSequence int64, limit int) ([]ruloResponseUpdate, int64, error) {
	if limit < 1 || limit > 250 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT u.sequence, r.response_id, e.id, u.version, u.signal, u.text, r.created_at, u.updated_at, e.feed_id, e.stream_id
		FROM rulo_response_updates u JOIN rulo_responses r ON r.response_id = u.response_id JOIN events e ON e.id = r.event_id
		WHERE u.sequence > ? ORDER BY u.sequence LIMIT ?`, afterSequence, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	updates := make([]ruloResponseUpdate, 0, limit)
	for rows.Next() {
		var update ruloResponseUpdate
		if err := rows.Scan(&update.Sequence, &update.Response.ResponseID, &update.Response.EventID, &update.Response.Version, &update.Response.Signal, &update.Response.Text, &update.Response.CreatedAt, &update.Response.UpdatedAt, &update.Response.FeedID, &update.Response.StreamID); err != nil {
			return nil, 0, err
		}
		updates = append(updates, update)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	var cursor int64
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM rulo_response_updates`).Scan(&cursor)
	return updates, cursor, err
}

func (s *eventStore) AttachRuloResponses(ctx context.Context, events []syntheticEvent) error {
	if len(events) == 0 {
		return nil
	}
	placeholders := make([]string, len(events))
	args := make([]any, len(events))
	for index, event := range events {
		placeholders[index] = "?"
		args[index] = event.ID
	}
	query := `SELECT r.event_id, r.response_id, r.version, r.signal, r.text, r.created_at, r.updated_at, e.feed_id, e.stream_id
		FROM rulo_responses r JOIN events e ON e.id = r.event_id WHERE r.event_id IN (` + strings.Join(placeholders, ",") + ")"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	byEvent := make(map[int64]*ruloPresentation, len(events))
	for rows.Next() {
		var eventID int64
		var response ruloPresentation
		if err := rows.Scan(&eventID, &response.ResponseID, &response.Version, &response.Signal, &response.Text, &response.CreatedAt, &response.UpdatedAt, &response.FeedID, &response.StreamID); err != nil {
			return err
		}
		response.EventID = eventID
		byEvent[eventID] = &response
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for index := range events {
		events[index].RuloResponse = byEvent[events[index].ID]
	}
	return nil
}

func (s *eventStore) MaxRuloResponseSequence(ctx context.Context) (int64, error) {
	var cursor int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM rulo_response_updates`).Scan(&cursor)
	return cursor, err
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
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := s.AttachRuloResponses(ctx, events); err != nil {
		return nil, err
	}
	// El panel muestra primero el comentario más antiguo dentro de la ventana.
	for left, right := 0, len(events)-1; left < right; left, right = left+1, right-1 {
		events[left], events[right] = events[right], events[left]
	}
	return events, nil
}

func (s *eventStore) After(ctx context.Context, afterID int64, limit int) ([]syntheticEvent, error) {
	if limit < 1 || limit > 1000 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, feed_id, source_id, external_id, stream_id, author, message, event_at, captured_at
		FROM events WHERE id > ? ORDER BY id ASC LIMIT ?`, afterID, limit)
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
	return events, nil
}

func (s *eventStore) EventByID(ctx context.Context, id int64) (syntheticEvent, error) {
	var event syntheticEvent
	err := s.db.QueryRowContext(ctx, `SELECT id, feed_id, source_id, external_id, stream_id, author, message, event_at, captured_at
		FROM events WHERE id = ?`, id).Scan(
		&event.ID, &event.FeedID, &event.SourceID, &event.ExternalID, &event.StreamID,
		&event.Author, &event.Message, &event.EventAt, &event.CreatedAt)
	return event, err
}

func (s *eventStore) Page(ctx context.Context, feedID, query, streamID, fromAt, toBefore string, limit, offset int) ([]syntheticEvent, int, error) {
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	where, args := eventWhere(feedID, query, streamID, fromAt, toBefore)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, feed_id, source_id, external_id, stream_id, author, message, event_at, captured_at
		FROM events`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	events := make([]syntheticEvent, 0)
	for rows.Next() {
		var event syntheticEvent
		if err := rows.Scan(&event.ID, &event.FeedID, &event.SourceID, &event.ExternalID, &event.StreamID,
			&event.Author, &event.Message, &event.EventAt, &event.CreatedAt); err != nil {
			return nil, 0, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := s.AttachRuloResponses(ctx, events); err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func eventWhere(feedID, query, streamID, fromAt, toBefore string) (string, []any) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 9)
	if feedID != "" {
		clauses = append(clauses, "feed_id = ?")
		args = append(args, feedID)
	}
	if streamID != "" {
		clauses = append(clauses, "stream_id = ?")
		args = append(args, streamID)
	}
	if query != "" {
		clauses = append(clauses, `(instr(lower(author), lower(?)) > 0 OR instr(lower(message), lower(?)) > 0 OR
			instr(lower(source_id), lower(?)) > 0 OR instr(lower(stream_id), lower(?)) > 0)`)
		args = append(args, query, query, query, query)
	}
	if fromAt != "" {
		clauses = append(clauses, "event_at >= ?")
		args = append(args, fromAt)
	}
	if toBefore != "" {
		clauses = append(clauses, "event_at < ?")
		args = append(args, toBefore)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (s *eventStore) CountAfter(ctx context.Context, feedID, query, streamID, fromAt, toBefore string, afterID int64) (int, error) {
	where, args := eventWhere(feedID, query, streamID, fromAt, toBefore)
	if where == "" {
		where = " WHERE id > ?"
	} else {
		where += " AND id > ?"
	}
	args = append(args, afterID)
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`+where, args...).Scan(&count)
	return count, err
}

func (s *eventStore) MaxEventID(ctx context.Context) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) FROM events`).Scan(&id)
	return id, err
}

func (s *eventStore) ExportPage(ctx context.Context, feedID, query, streamID, fromAt, toBefore string, maxID, beforeID int64, limit int) ([]syntheticEvent, error) {
	if limit < 1 || limit > 1000 {
		limit = 1000
	}
	where, args := eventWhere(feedID, query, streamID, fromAt, toBefore)
	if where == "" {
		where = " WHERE id <= ?"
	} else {
		where += " AND id <= ?"
	}
	args = append(args, maxID)
	if beforeID > 0 {
		where += " AND id < ?"
		args = append(args, beforeID)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `SELECT id, feed_id, source_id, external_id, stream_id, author, message, event_at, captured_at
		FROM events`+where+` ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]syntheticEvent, 0, limit)
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
	return events, nil
}

type eventArchive struct {
	ID        int64            `json:"id"`
	Program   string           `json:"program"`
	Comments  int              `json:"comments"`
	CreatedAt string           `json:"createdAt"`
	Events    []syntheticEvent `json:"events,omitempty"`
}

type eventStatistics struct {
	From              string              `json:"from"`
	To                string              `json:"to"`
	FeedID            string              `json:"feedId,omitempty"`
	ActiveComments    int64               `json:"activeComments"`
	ActiveByFeed      []eventCountByFeed  `json:"activeByFeed"`
	ActiveByDay       []eventCountByDay   `json:"activeByDay"`
	SavedProgramCount int64               `json:"savedProgramCount"`
	SavedCommentCount int64               `json:"savedCommentCount"`
	SavedPrograms     []eventArchiveStats `json:"savedPrograms"`
}

type eventCountByFeed struct {
	FeedID string `json:"feedId"`
	Count  int64  `json:"count"`
}

type eventCountByDay struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type eventArchiveStats struct {
	Program   string `json:"program"`
	Comments  int64  `json:"comments"`
	CreatedAt string `json:"createdAt"`
}

func (s *eventStore) ArchiveExists(ctx context.Context, program string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM event_archives WHERE program = ?)`, program).Scan(&exists)
	return exists, err
}

func (s *eventStore) SaveArchive(ctx context.Context, program string, events []syntheticEvent) (eventArchive, error) {
	encoded, err := json.Marshal(events)
	if err != nil {
		return eventArchive{}, err
	}
	if len(encoded) > 8<<20 {
		return eventArchive{}, errors.New("el chat guardado supera el límite de 8 MiB")
	}
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `INSERT INTO event_archives(program, events_json, event_count, created_at) VALUES(?, ?, ?, ?)`,
		program, string(encoded), len(events), createdAt)
	if err != nil {
		return eventArchive{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return eventArchive{}, err
	}
	return eventArchive{ID: id, Program: program, Comments: len(events), CreatedAt: createdAt}, nil
}

func (s *eventStore) Archives(ctx context.Context) ([]eventArchive, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, program, event_count, created_at FROM event_archives ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	archives := make([]eventArchive, 0)
	for rows.Next() {
		var archive eventArchive
		if err := rows.Scan(&archive.ID, &archive.Program, &archive.Comments, &archive.CreatedAt); err != nil {
			return nil, err
		}
		archives = append(archives, archive)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return archives, nil
}

func (s *eventStore) Archive(ctx context.Context, id int64) (eventArchive, error) {
	var archive eventArchive
	var encoded string
	err := s.db.QueryRowContext(ctx, `SELECT id, program, event_count, created_at, events_json FROM event_archives WHERE id = ?`, id).Scan(
		&archive.ID, &archive.Program, &archive.Comments, &archive.CreatedAt, &encoded)
	if err != nil {
		return eventArchive{}, err
	}
	if err := json.Unmarshal([]byte(encoded), &archive.Events); err != nil {
		return eventArchive{}, fmt.Errorf("leer el chat guardado: %w", err)
	}
	return archive, nil
}

func (s *eventStore) DeleteArchive(ctx context.Context, id int64) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM event_archives WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *eventStore) Count(ctx context.Context, feedID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE feed_id = ?`, feedID).Scan(&count)
	return count, err
}

func (s *eventStore) Statistics(ctx context.Context, from, to, feedID string) (eventStatistics, error) {
	stats := eventStatistics{
		From: from, To: to, FeedID: feedID,
		ActiveByFeed:  make([]eventCountByFeed, 0, 2),
		ActiveByDay:   make([]eventCountByDay, 0),
		SavedPrograms: make([]eventArchiveStats, 0, 20),
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return eventStatistics{}, err
	}
	defer tx.Rollback()
	where := `source_id NOT LIKE 'test:%' AND event_at >= ? AND event_at < ?`
	args := []any{from, to}
	if feedID != "" {
		where += ` AND feed_id = ?`
		args = append(args, feedID)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE `+where, args...).Scan(&stats.ActiveComments); err != nil {
		return eventStatistics{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT feed_id, COUNT(*) FROM events WHERE `+where+` GROUP BY feed_id ORDER BY feed_id`, args...)
	if err != nil {
		return eventStatistics{}, err
	}
	for rows.Next() {
		var item eventCountByFeed
		if err := rows.Scan(&item.FeedID, &item.Count); err != nil {
			rows.Close()
			return eventStatistics{}, err
		}
		stats.ActiveByFeed = append(stats.ActiveByFeed, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return eventStatistics{}, err
	}
	rows.Close()
	rows, err = tx.QueryContext(ctx, `SELECT substr(event_at, 1, 10) AS day, COUNT(*) FROM events WHERE `+where+` GROUP BY day ORDER BY day`, args...)
	if err != nil {
		return eventStatistics{}, err
	}
	for rows.Next() {
		var item eventCountByDay
		if err := rows.Scan(&item.Date, &item.Count); err != nil {
			rows.Close()
			return eventStatistics{}, err
		}
		stats.ActiveByDay = append(stats.ActiveByDay, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return eventStatistics{}, err
	}
	rows.Close()
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(event_count), 0) FROM event_archives`).Scan(&stats.SavedProgramCount, &stats.SavedCommentCount); err != nil {
		return eventStatistics{}, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT program, event_count, created_at FROM event_archives ORDER BY event_count DESC, id DESC LIMIT 20`)
	if err != nil {
		return eventStatistics{}, err
	}
	for rows.Next() {
		var item eventArchiveStats
		if err := rows.Scan(&item.Program, &item.Comments, &item.CreatedAt); err != nil {
			rows.Close()
			return eventStatistics{}, err
		}
		stats.SavedPrograms = append(stats.SavedPrograms, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return eventStatistics{}, err
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		return eventStatistics{}, err
	}
	return stats, nil
}

func (s *eventStore) Delete(ctx context.Context, id int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM event_dispatch WHERE event_id = ?`, id); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return affected > 0, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *eventStore) DeleteAll(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM event_dispatch`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM events`); err != nil {
		return err
	}
	return tx.Commit()
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
