package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestFeedsKeepIndependentStatusAndPersistEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synthetic.sqlite")
	store, err := openEventStore(path)
	if err != nil {
		t.Fatal(err)
	}
	service := newWorkerService(store, func(context.Context) obsStatus {
		return obsStatus{State: "disabled", Label: "OBS de prueba omitido", ReadOnly: true}
	})
	mux := http.NewServeMux()
	service.registerRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	defer store.Close()

	postJSON(t, server.URL+"/api/simulate/comment", `{"feedId":"a","author":"Cuenta A","message":"evento de prueba A"}`, http.StatusCreated)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/feeds/a/disconnect", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("desconexión de prueba devolvió %s", response.Status)
	}

	statusResponse, err := http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer statusResponse.Body.Close()
	var status workerStatus
	if err := json.NewDecoder(statusResponse.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if len(status.Feeds) != 2 || status.Feeds[0].Connected || !status.Feeds[1].Connected {
		t.Fatalf("desconectar A también afectó B o el estado es inválido: %#v", status.Feeds)
	}
	if status.Storage.EventCount != 1 {
		t.Fatalf("se esperaba un evento guardado; estado SQLite: %#v", status.Storage)
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
	if len(events) != 1 || events[0].Message != "evento de prueba A" {
		t.Fatalf("el evento no sobrevivió a la reapertura: %#v", events)
	}
}

func postJSON(t *testing.T, url, payload string, expectedStatus int) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != expectedStatus {
		message, _ := io.ReadAll(response.Body)
		t.Fatalf("respuesta %s, se esperaba %d: %s", response.Status, expectedStatus, string(message))
	}
}
