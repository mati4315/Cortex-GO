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
