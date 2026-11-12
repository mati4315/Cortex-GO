package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSyntheticSQLiteBackupAndRestore(t *testing.T) {
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "active.sqlite")
	backupPath := filepath.Join(directory, "snapshots", "before-change.sqlite")
	store, err := openEventStore(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, err := store.Add(ctx, syntheticEvent{FeedID: "a", Author: "Prueba A", Message: "antes del respaldo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(ctx, syntheticEvent{FeedID: "b", Author: "Prueba B", Message: "segundo evento"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Backup(ctx, backupPath); err != nil {
		t.Fatalf("crear respaldo sintético: %v", err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("no se publicó el archivo de respaldo: %v", err)
	}

	if _, err := store.Add(ctx, syntheticEvent{FeedID: "a", Author: "Prueba A", Message: "después del respaldo"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Restore(ctx, backupPath); err != nil {
		t.Fatalf("restaurar respaldo sintético: %v", err)
	}

	total, err := store.Total(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("se esperaban 2 eventos tras restaurar, se encontraron %d", total)
	}
	events, err := store.Recent(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Message != "antes del respaldo" || events[1].Message != "segundo evento" {
		t.Fatalf("la restauración no recuperó el snapshot esperado: %#v", events)
	}
}

func TestSyntheticSQLiteBackupDoesNotOverwriteExistingFile(t *testing.T) {
	directory := t.TempDir()
	store, err := openEventStore(filepath.Join(directory, "active.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	backupPath := filepath.Join(directory, "existing.sqlite")
	sentinel := []byte("archivo que debe preservarse")
	if err := os.WriteFile(backupPath, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Backup(context.Background(), backupPath); err == nil {
		t.Fatal("se esperaba rechazar un destino de respaldo existente")
	}
	contents, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != string(sentinel) {
		t.Fatalf("el archivo existente cambió: %q", contents)
	}
}

func TestSyntheticSQLiteRestoreRejectsCorruptSnapshotWithoutChangingDatabase(t *testing.T) {
	directory := t.TempDir()
	store, err := openEventStore(filepath.Join(directory, "active.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, err := store.Add(ctx, syntheticEvent{FeedID: "a", Author: "Prueba", Message: "dato vigente"}); err != nil {
		t.Fatal(err)
	}
	corruptSnapshot := filepath.Join(directory, "corrupt.sqlite")
	if err := os.WriteFile(corruptSnapshot, []byte("esto no es una base SQLite"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Restore(ctx, corruptSnapshot); err == nil {
		t.Fatal("se esperaba rechazar un respaldo corrupto")
	}

	total, err := store.Total(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("la restauración inválida modificó la base activa: quedaron %d eventos", total)
	}
	latest, err := store.Recent(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 1 || latest[0].Message != "dato vigente" {
		t.Fatalf("la base activa no conservó el dato anterior: %#v", latest)
	}
}
