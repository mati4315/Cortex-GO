package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
)

type sqliteBackupConnection interface {
	NewBackup(destinationURI string) (*sqlite.Backup, error)
	NewRestore(sourceURI string) (*sqlite.Backup, error)
}

// Backup creates a new, validated snapshot without replacing an existing file.
// The destination must be on a local filesystem, not the Taildrive/WebDAV share.
func (s *eventStore) Backup(ctx context.Context, destination string) error {
	if s == nil || s.db == nil {
		return errors.New("la base de datos no está abierta")
	}
	destinationPath, err := absoluteFilePath(destination)
	if err != nil {
		return fmt.Errorf("ruta del respaldo: %w", err)
	}
	if sameSQLiteFilePath(s.path, destinationPath) {
		return errors.New("el destino del respaldo coincide con la base activa")
	}
	if err := requirePathAbsent(destinationPath); err != nil {
		return fmt.Errorf("destino del respaldo: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
		return fmt.Errorf("preparar carpeta del respaldo: %w", err)
	}

	temporaryFile, err := os.CreateTemp(filepath.Dir(destinationPath), ".cortex-backup-*.sqlite")
	if err != nil {
		return fmt.Errorf("crear archivo temporal del respaldo: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	if err := temporaryFile.Close(); err != nil {
		removeSQLiteFiles(temporaryPath)
		return fmt.Errorf("cerrar archivo temporal del respaldo: %w", err)
	}
	published := false
	defer func() {
		if !published {
			removeSQLiteFiles(temporaryPath)
		}
	}()

	connection, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reservar conexión SQLite para respaldo: %w", err)
	}
	backupErr := connection.Raw(func(rawConnection any) error {
		driverConnection, ok := rawConnection.(sqliteBackupConnection)
		if !ok {
			return errors.New("el controlador SQLite no ofrece la API de respaldo en línea")
		}
		backup, err := driverConnection.NewBackup(temporaryPath)
		if err != nil {
			return fmt.Errorf("iniciar respaldo SQLite: %w", err)
		}
		stepErr := finishSQLiteCopy(backup)
		finishErr := backup.Finish()
		if stepErr != nil {
			return stepErr
		}
		if finishErr != nil {
			return fmt.Errorf("finalizar respaldo SQLite: %w", finishErr)
		}
		return nil
	})
	closeErr := connection.Close()
	if backupErr != nil {
		return backupErr
	}
	if closeErr != nil {
		return fmt.Errorf("liberar conexión SQLite del respaldo: %w", closeErr)
	}
	if err := validateSQLiteSnapshot(ctx, temporaryPath); err != nil {
		return fmt.Errorf("validar respaldo SQLite: %w", err)
	}
	if err := requirePathAbsent(destinationPath); err != nil {
		return fmt.Errorf("publicar respaldo sin sobrescribir otro archivo: %w", err)
	}
	if err := os.Rename(temporaryPath, destinationPath); err != nil {
		return fmt.Errorf("publicar respaldo SQLite: %w", err)
	}
	published = true
	return nil
}

// Restore replaces the open database with a validated snapshot. It first keeps
// a temporary rollback snapshot and restores that snapshot if validation fails.
func (s *eventStore) Restore(ctx context.Context, source string) error {
	if s == nil || s.db == nil {
		return errors.New("la base de datos no está abierta")
	}
	sourcePath, err := absoluteFilePath(source)
	if err != nil {
		return fmt.Errorf("ruta de restauración: %w", err)
	}
	if sameSQLiteFilePath(s.path, sourcePath) {
		return errors.New("la fuente de restauración coincide con la base activa")
	}
	if err := validateSQLiteSnapshot(ctx, sourcePath); err != nil {
		return fmt.Errorf("el respaldo no es válido para Cortex Go: %w", err)
	}

	rollbackPath, err := unusedTemporaryPath(filepath.Dir(s.path), ".cortex-rollback-*.sqlite")
	if err != nil {
		return fmt.Errorf("preparar punto de retorno: %w", err)
	}
	defer removeSQLiteFiles(rollbackPath)
	if err := s.Backup(ctx, rollbackPath); err != nil {
		return fmt.Errorf("crear punto de retorno antes de restaurar: %w", err)
	}

	if err := s.applySQLiteSnapshot(ctx, sourcePath); err != nil {
		return s.rollbackRestore(rollbackPath, fmt.Errorf("restaurar respaldo: %w", err))
	}
	if err := migrateEventSchema(s.db); err != nil {
		return s.rollbackRestore(rollbackPath, fmt.Errorf("actualizar el esquema del respaldo: %w", err))
	}
	if err := s.prepareRestoredRuntime(ctx); err != nil {
		return s.rollbackRestore(rollbackPath, fmt.Errorf("preparar el runtime restaurado: %w", err))
	}
	if err := validateSQLiteDatabase(ctx, s.db); err != nil {
		return s.rollbackRestore(rollbackPath, fmt.Errorf("validar base restaurada: %w", err))
	}
	return nil
}

func (s *eventStore) prepareRestoredRuntime(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	settings := map[string]json.RawMessage{}
	var raw string
	err = tx.QueryRowContext(ctx, `SELECT settings_json FROM module_settings WHERE module_id=?`, spotifyModuleID).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		if err := json.Unmarshal([]byte(raw), &settings); err != nil {
			return fmt.Errorf("la configuración Spotify del respaldo no es válida: %w", err)
		}
	}
	if settings == nil {
		settings = map[string]json.RawMessage{}
	}
	settings["liveQueueEnabled"] = json.RawMessage("false")
	var recentMinutes int
	if err := json.Unmarshal(settings["recentOrderMinutes"], &recentMinutes); err != nil || recentMinutes < 1 || recentMinutes > 30 {
		settings["recentOrderMinutes"] = json.RawMessage("10")
	}
	var clientID string
	if err := json.Unmarshal(settings["clientId"], &clientID); err != nil || (clientID != "" && !validSpotifyClientID(clientID)) {
		settings["clientId"] = json.RawMessage(`""`)
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO module_settings(module_id,settings_json,updated_at) VALUES(?,?,?)
		ON CONFLICT(module_id) DO UPDATE SET settings_json=excluded.settings_json,updated_at=excluded.updated_at`,
		spotifyModuleID, string(encoded), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}

	var latest int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence),0) FROM event_dispatch`).Scan(&latest); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO dispatch_cursors(consumer_id,sequence,updated_at) VALUES(?,?,?)
		ON CONFLICT(consumer_id) DO UPDATE SET sequence=MAX(sequence,excluded.sequence),updated_at=excluded.updated_at`,
		ruloConsumerID, latest, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE spotify_operations SET status='incierto',detail='Se restauró una copia después del envío; el resultado no se reintenta automáticamente.',updated_at=? WHERE status='pendiente' AND sent_at<>''`, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE spotify_operations SET status='cancelado',detail='La restauración omitió operaciones aún no enviadas para evitar reproducir pedidos históricos.',updated_at=? WHERE status='pendiente' AND sent_at=''`, now); err != nil {
		return err
	}
	var minimum int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MIN(sequence),0) FROM dispatch_cursors`).Scan(&minimum); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM event_dispatch WHERE sequence<=?`, minimum); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *eventStore) applySQLiteSnapshot(ctx context.Context, sourcePath string) error {
	connection, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reservar conexión SQLite para restaurar: %w", err)
	}
	defer connection.Close()

	return connection.Raw(func(rawConnection any) error {
		driverConnection, ok := rawConnection.(sqliteBackupConnection)
		if !ok {
			return errors.New("el controlador SQLite no ofrece la API de restauración en línea")
		}
		backup, err := driverConnection.NewRestore(sourcePath)
		if err != nil {
			return fmt.Errorf("abrir respaldo SQLite: %w", err)
		}
		stepErr := finishSQLiteCopy(backup)
		finishErr := backup.Finish()
		if stepErr != nil {
			return stepErr
		}
		if finishErr != nil {
			return fmt.Errorf("finalizar restauración SQLite: %w", finishErr)
		}
		return nil
	})
}

func (s *eventStore) rollbackRestore(rollbackPath string, restoreErr error) error {
	if err := s.applySQLiteSnapshot(context.Background(), rollbackPath); err != nil {
		return errors.Join(restoreErr, fmt.Errorf("restaurar la base previa: %w", err))
	}
	return fmt.Errorf("%w; se recuperó la base previa", restoreErr)
}

func finishSQLiteCopy(backup *sqlite.Backup) error {
	for more := true; more; {
		var err error
		more, err = backup.Step(-1)
		if err != nil {
			return fmt.Errorf("copiar páginas SQLite: %w", err)
		}
	}
	return nil
}

func validateSQLiteSnapshot(ctx context.Context, path string) error {
	absolutePath, err := absoluteFilePath(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return fmt.Errorf("abrir archivo de respaldo: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("el respaldo no es un archivo normal")
	}

	db, err := sql.Open("sqlite", absolutePath)
	if err != nil {
		return fmt.Errorf("abrir SQLite de solo lectura: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA query_only=ON"); err != nil {
		return fmt.Errorf("proteger el snapshot contra escrituras: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("comprobar archivo SQLite: %w", err)
	}
	return validateSQLiteDatabase(ctx, db)
}

func validateSQLiteDatabase(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("comprobar integridad SQLite: %w", err)
	}
	defer rows.Close()
	resultCount := 0
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("leer resultado de integridad SQLite: %w", err)
		}
		resultCount++
		if result != "ok" {
			return fmt.Errorf("integridad SQLite: %s", result)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("leer comprobación de integridad SQLite: %w", err)
	}
	if resultCount != 1 {
		return fmt.Errorf("resultado inesperado de integridad SQLite: %d filas", resultCount)
	}

	var eventsTableExists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='events')`).Scan(&eventsTableExists); err != nil {
		return fmt.Errorf("comprobar esquema de eventos: %w", err)
	}
	if eventsTableExists {
		var eventCount int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&eventCount); err != nil {
			return fmt.Errorf("leer registro de eventos: %w", err)
		}
		return nil
	}
	// Los snapshots hechos antes de la migración siguen siendo importables;
	// Restore actualiza su esquema después de aplicarlos.
	var legacyTableExists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='synthetic_events')`).Scan(&legacyTableExists); err != nil {
		return fmt.Errorf("comprobar esquema anterior: %w", err)
	}
	if legacyTableExists {
		var eventCount int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM synthetic_events`).Scan(&eventCount); err != nil {
			return fmt.Errorf("leer eventos anteriores: %w", err)
		}
		return nil
	}
	return errors.New("el archivo no contiene el esquema de eventos de Cortex Go")
}

func absoluteFilePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("la ruta está vacía")
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolutePath), nil
}

func sameSQLiteFilePath(first, second string) bool {
	if strings.EqualFold(filepath.Clean(first), filepath.Clean(second)) {
		return true
	}
	firstInfo, firstErr := os.Stat(first)
	secondInfo, secondErr := os.Stat(second)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}

func requirePathAbsent(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return os.ErrExist
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func unusedTemporaryPath(directory, pattern string) (string, error) {
	file, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		removeSQLiteFiles(path)
		return "", err
	}
	if err := os.Remove(path); err != nil {
		removeSQLiteFiles(path)
		return "", err
	}
	return path, nil
}

func removeSQLiteFiles(path string) {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(path + suffix)
	}
}
