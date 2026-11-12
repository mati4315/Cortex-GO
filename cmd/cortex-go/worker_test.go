package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
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

func TestCapturedEventAPIIsIdempotentAndSupportsManualDeletion(t *testing.T) {
	store, err := openEventStore(filepath.Join(t.TempDir(), "captured.sqlite"))
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

	payload := `{"feedId":"a","sourceId":"facebook:cuenta-1","externalId":"comment-42","streamId":"live-42","author":"Autora","message":"hola","eventAt":"2026-11-13T05:00:00Z"}`
	postJSON(t, server.URL+"/api/events", payload, http.StatusCreated)
	duplicatePayload := `{"feedId":"a","sourceId":"facebook:cuenta-1","externalId":"comment-42","streamId":"live-42","author":"Autora","message":"hola re-renderizado","eventAt":"2026-11-13T05:00:00Z"}`
	postJSON(t, server.URL+"/api/events", duplicatePayload, http.StatusOK)

	eventsResponse, err := http.Get(server.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	var events struct {
		Events []syntheticEvent `json:"events"`
	}
	if err := json.NewDecoder(eventsResponse.Body).Decode(&events); err != nil {
		eventsResponse.Body.Close()
		t.Fatal(err)
	}
	eventsResponse.Body.Close()
	if len(events.Events) != 1 || events.Events[0].Message != "hola" || events.Events[0].SourceID != "facebook:cuenta-1" {
		t.Fatalf("el historial contiene un duplicado o no conservó el primer comentario: %#v", events.Events)
	}

	deleteRequest, err := http.NewRequest(http.MethodDelete, server.URL+"/api/events/"+strconv.FormatInt(events.Events[0].ID, 10), nil)
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse, err := http.DefaultClient.Do(deleteRequest)
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse.Body.Close()
	if deleteResponse.StatusCode != http.StatusOK {
		t.Fatalf("borrar un comentario devolvió %s", deleteResponse.Status)
	}

	postJSON(t, server.URL+"/api/events", duplicatePayload, http.StatusCreated)
	deleteAllRequest, err := http.NewRequest(http.MethodDelete, server.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	deleteAllResponse, err := http.DefaultClient.Do(deleteAllRequest)
	if err != nil {
		t.Fatal(err)
	}
	deleteAllResponse.Body.Close()
	if deleteAllResponse.StatusCode != http.StatusOK {
		t.Fatalf("borrar todo el historial devolvió %s", deleteAllResponse.Status)
	}
	total, err := store.Total(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("el borrado manual dejó %d eventos activos", total)
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
