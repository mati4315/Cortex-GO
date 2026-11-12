package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	if err := validateSQLiteDatabase(ctx, s.db); err != nil {
		return s.rollbackRestore(rollbackPath, fmt.Errorf("validar base restaurada: %w", err))
	}
	return nil
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

	var eventCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM synthetic_events`).Scan(&eventCount); err != nil {
		return fmt.Errorf("comprobar esquema de eventos sintéticos: %w", err)
	}
	return nil
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
