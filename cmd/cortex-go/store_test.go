package main

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSyntheticEventsSurviveStoreReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synthetic.sqlite")
	store, err := openEventStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(context.Background(), syntheticEvent{FeedID: "a", Author: "Prueba", Message: "comentario ficticio"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := openEventStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	events, err := reopened.Recent(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Message != "comentario ficticio" || events[0].FeedID != "a" {
		t.Fatalf("evento no persistió como se esperaba: %#v", events)
	}
}

func TestCapturedEventsSurviveReopenAndDeduplicateBySourceID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "captured.sqlite")
	event := capturedEvent{
		FeedID: "a", SourceID: "facebook:cuenta-1", ExternalID: "comment-42",
		StreamID: "live-42", Author: "Autora", Message: "comentario real de prueba",
		EventAt: "2026-11-13T05:00:00Z", CapturedAt: "2026-11-13T05:00:01Z", DedupKey: "id:comment-42",
	}
	store, err := openEventStore(path)
	if err != nil {
		t.Fatal(err)
	}
	first, inserted, err := store.AddCaptured(context.Background(), event)
	if err != nil || !inserted {
		t.Fatalf("no se guardó el primer evento: inserted=%v err=%v", inserted, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := openEventStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	duplicate := event
	duplicate.Message = "texto re-renderizado"
	duplicate.CapturedAt = "2026-11-13T05:00:05Z"
	stored, inserted, err := reopened.AddCaptured(context.Background(), duplicate)
	if err != nil || inserted {
		t.Fatalf("el evento repetido se insertó: inserted=%v err=%v", inserted, err)
	}
	if stored.ID != first.ID || stored.Message != event.Message {
		t.Fatalf("la deduplicación no devolvió el evento persistido original: %#v", stored)
	}
	total, err := reopened.Total(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("la base conserva %d filas; se esperaba una sola", total)
	}
}
