package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupAndRestoreHTTPHandlers(t *testing.T) {
	dataDirectory := t.TempDir()
	store, err := openEventStore(filepath.Join(dataDirectory, "first-cut.sqlite"))
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

	postJSON(t, server.URL+"/api/simulate/comment", `{"feedId":"a","author":"Cuenta A","message":"evento conservado"}`, http.StatusCreated)

	backupResponse, err := http.Post(server.URL+"/api/backup", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	var backup struct {
		FileName string `json:"fileName"`
		Location string `json:"location"`
		State    string `json:"state"`
	}
	if err := json.NewDecoder(backupResponse.Body).Decode(&backup); err != nil {
		backupResponse.Body.Close()
		t.Fatal(err)
	}
	backupResponse.Body.Close()
	if backupResponse.StatusCode != http.StatusCreated || backup.State != "created" || backup.FileName == "" {
		t.Fatalf("respuesta de respaldo inesperada: status=%s payload=%#v", backupResponse.Status, backup)
	}
	backupPath := filepath.Join(dataDirectory, backup.Location)
	backupBytes, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("no se encontró el respaldo local %q: %v", backupPath, err)
	}

	postJSON(t, server.URL+"/api/simulate/comment", `{"feedId":"b","author":"Cuenta B","message":"evento posterior"}`, http.StatusCreated)

	restoreResponse := postBackupFile(t, server.URL+"/api/restore", backup.FileName, backupBytes, http.StatusOK)
	var restored struct {
		State             string `json:"state"`
		BeforeRestoreFile string `json:"beforeRestoreFile"`
	}
	if err := json.NewDecoder(restoreResponse.Body).Decode(&restored); err != nil {
		restoreResponse.Body.Close()
		t.Fatal(err)
	}
	restoreResponse.Body.Close()
	if restored.State != "restored" || restored.BeforeRestoreFile == "" {
		t.Fatalf("respuesta de restauración inesperada: %#v", restored)
	}
	if _, err := os.Stat(filepath.Join(dataDirectory, "backups", restored.BeforeRestoreFile)); err != nil {
		t.Fatalf("no se conservó la copia automática previa: %v", err)
	}

	response, err := http.Get(server.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var events struct {
		Events []syntheticEvent `json:"events"`
	}
	if err := json.NewDecoder(response.Body).Decode(&events); err != nil {
		t.Fatal(err)
	}
	if len(events.Events) != 1 || events.Events[0].Message != "evento conservado" {
		t.Fatalf("la API no restauró el contenido del snapshot: %#v", events.Events)
	}
}

func TestRestoreHTTPHandlerRejectsInvalidSQLiteFile(t *testing.T) {
	dataDirectory := t.TempDir()
	store, err := openEventStore(filepath.Join(dataDirectory, "first-cut.sqlite"))
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

	postJSON(t, server.URL+"/api/simulate/comment", `{"feedId":"a","author":"Cuenta A","message":"dato actual"}`, http.StatusCreated)
	response := postBackupFile(t, server.URL+"/api/restore", "incorrecto.sqlite", []byte("no es SQLite"), http.StatusUnprocessableEntity)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("no es un respaldo SQLite válido")) {
		t.Fatalf("la respuesta no explica por qué se rechazó el archivo: %s", body)
	}
	total, err := store.Total(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("el archivo inválido modificó la base activa: quedaron %d eventos", total)
	}
}

func TestBackupListAndDeleteHandlersOnlyRemoveManagedSnapshots(t *testing.T) {
	dataDirectory := t.TempDir()
	store, err := openEventStore(filepath.Join(dataDirectory, "first-cut.sqlite"))
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

	firstName, _, err := service.createLocalBackup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secondName, _, err := service.createLocalBackup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	backupDirectory := filepath.Join(dataDirectory, "backups")
	foreignPath := filepath.Join(backupDirectory, "manual-note.sqlite")
	foreignContents := []byte("archivo que Cortex no debe administrar")
	if err := os.WriteFile(foreignPath, foreignContents, 0o600); err != nil {
		t.Fatal(err)
	}

	listResponse, err := http.Get(server.URL + "/api/backups")
	if err != nil {
		t.Fatal(err)
	}
	var listed struct {
		Backups []backupFileStatus `json:"backups"`
	}
	if err := json.NewDecoder(listResponse.Body).Decode(&listed); err != nil {
		listResponse.Body.Close()
		t.Fatal(err)
	}
	listResponse.Body.Close()
	if listResponse.StatusCode != http.StatusOK || len(listed.Backups) != 2 {
		t.Fatalf("la lista de respaldos no coincide: status=%s backups=%#v", listResponse.Status, listed.Backups)
	}

	deleteRequest, err := http.NewRequest(http.MethodDelete, server.URL+"/api/backups/"+firstName, nil)
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse, err := http.DefaultClient.Do(deleteRequest)
	if err != nil {
		t.Fatal(err)
	}
	deleteResponse.Body.Close()
	if deleteResponse.StatusCode != http.StatusOK {
		t.Fatalf("el borrado individual devolvió %s", deleteResponse.Status)
	}
	if _, err := os.Stat(filepath.Join(backupDirectory, firstName)); !os.IsNotExist(err) {
		t.Fatalf("el respaldo individual sigue en la carpeta: %v", err)
	}

	bulkResponse, err := http.NewRequest(http.MethodDelete, server.URL+"/api/backups", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(bulkResponse)
	if err != nil {
		t.Fatal(err)
	}
	var deleted struct {
		State        string   `json:"state"`
		DeletedCount int      `json:"deletedCount"`
		FailedFiles  []string `json:"failedFiles"`
	}
	if err := json.NewDecoder(response.Body).Decode(&deleted); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || deleted.State != "deleted" || deleted.DeletedCount != 1 || len(deleted.FailedFiles) != 0 {
		t.Fatalf("el borrado total devolvió estado inesperado: status=%s body=%#v", response.Status, deleted)
	}
	if _, err := os.Stat(filepath.Join(backupDirectory, secondName)); !os.IsNotExist(err) {
		t.Fatalf("el segundo respaldo sigue en la carpeta: %v", err)
	}
	actualForeignContents, err := os.ReadFile(foreignPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(actualForeignContents) != string(foreignContents) {
		t.Fatalf("el borrado de respaldos alteró un archivo ajeno: %q", actualForeignContents)
	}
}

func TestManagedBackupNameRejectsPathComponents(t *testing.T) {
	valid := "CortexGo-20261113-153000-123456789-004.sqlite"
	if !isManagedBackupName(valid) {
		t.Fatalf("se rechazó un nombre de respaldo válido: %q", valid)
	}
	for _, invalid := range []string{
		"../" + valid,
		`..\` + valid,
		"CortexGo-latest.sqlite",
		"manual-note.sqlite",
	} {
		if isManagedBackupName(invalid) {
			t.Errorf("se aceptó un nombre no administrado: %q", invalid)
		}
	}
}

func postBackupFile(t *testing.T, url, filename string, contents []byte, expectedStatus int) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("backup", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != expectedStatus {
		responseBody, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("restauración devolvió %s, se esperaba %d: %s", response.Status, expectedStatus, responseBody)
	}
	return response
}
