package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxBackupUploadBytes = 128 << 20

type feedStatus struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	State                 string `json:"state"`
	Connected             bool   `json:"connected"`
	CaptureAgentConnected bool   `json:"captureAgentConnected"`
	CommentsPage          bool   `json:"commentsPage"`
	VisibleRows           int    `json:"visibleRows"`
	NewComments           int    `json:"newComments"`
	LastScanAt            string `json:"lastScanAt,omitempty"`
	LastDetectedAt        string `json:"lastDetectedAt,omitempty"`
	LastEventAt           string `json:"lastEventAt,omitempty"`
	EventCount            int    `json:"eventCount"`
	LastError             string `json:"lastError,omitempty"`
}

type storageStatus struct {
	State         string `json:"state"`
	SchemaVersion int    `json:"schemaVersion"`
	EventCount    int    `json:"eventCount"`
	LastSaved     string `json:"lastSaved,omitempty"`
}

type backupFileStatus struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modifiedAt"`
}

type workerStatus struct {
	Backend struct {
		State     string `json:"state"`
		StartedAt string `json:"startedAt"`
	} `json:"backend"`
	Storage storageStatus `json:"storage"`
	OBS     obsStatus     `json:"obs"`
	Feeds   []feedStatus  `json:"feeds"`
}

type workerService struct {
	store     *eventStore
	welcome   *welcomeService
	lights    *lightsService
	startedAt string
	probe     func(context.Context) obsStatus

	storageMu sync.Mutex
	mu        sync.RWMutex
	feeds     map[string]feedStatus
	obs       obsStatus
	lastSaved string
	stop      chan struct{}
	stopOnce  sync.Once
}

func newWorkerService(store *eventStore, probe func(context.Context) obsStatus) *workerService {
	now := time.Now().UTC().Format(time.RFC3339)
	return &workerService{
		store:     store,
		startedAt: now,
		probe:     probe,
		feeds: map[string]feedStatus{
			"a": {ID: "a", Name: "Cuenta 1", State: "no_iniciado", Connected: true},
			"b": {ID: "b", Name: "Cuenta 2", State: "no_iniciado", Connected: true},
		},
		obs:  obsStatus{State: "checking", Label: "Comprobando OBS", ReadOnly: true},
		stop: make(chan struct{}),
	}
}

func runWorker(address, dbPath, parentURL string) error {
	store, err := openEventStore(dbPath)
	if err != nil {
		return fmt.Errorf("no se pudo preparar la base sintética nueva: %w", err)
	}
	defer store.Close()
	service := newWorkerService(store, probeOBSFromEnvironment)
	service.welcome, err = newWelcomeService(store, switchOBSSceneFromEnvironment)
	if err != nil {
		return fmt.Errorf("no se pudo preparar el módulo Bienvenidos y Banner: %w", err)
	}
	service.lights, err = newLightsService(store)
	if err != nil {
		return fmt.Errorf("no se pudo preparar el módulo Luces: %w", err)
	}
	mux := http.NewServeMux()
	service.registerRoutes(mux)
	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 4 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("no se pudo escuchar en la dirección interna: %w", err)
	}
	go service.watchOBS()
	go service.welcome.run(service.stop)
	if parentURL != "" {
		go service.watchParent(parentURL)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	select {
	case <-service.stop:
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("el servidor interno se cerró: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("cierre ordenado del servidor interno: %w", err)
	}
	return nil
}

func (s *workerService) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("POST /api/events", s.handleCaptureEvent)
	mux.HandleFunc("DELETE /api/events", s.handleDeleteAllEvents)
	mux.HandleFunc("DELETE /api/events/{id}", s.handleDeleteEvent)
	mux.HandleFunc("POST /api/feeds/{id}/status", s.handleCaptureStatus)
	mux.HandleFunc("POST /api/simulate/comment", s.handleSimulatedComment)
	mux.HandleFunc("POST /api/backup", s.handleBackup)
	mux.HandleFunc("GET /api/backups", s.handleBackups)
	mux.HandleFunc("DELETE /api/backups", s.handleDeleteAllBackups)
	mux.HandleFunc("DELETE /api/backups/{name}", s.handleDeleteBackup)
	mux.HandleFunc("POST /api/restore", s.handleRestore)
	mux.HandleFunc("POST /api/feeds/{id}/disconnect", s.handleFeedDisconnect)
	mux.HandleFunc("POST /api/feeds/{id}/reconnect", s.handleFeedReconnect)
	mux.HandleFunc("GET /api/modules/welcome/state", s.handleWelcomeState)
	mux.HandleFunc("GET /api/modules/welcome/settings", s.welcome.handleSettings)
	mux.HandleFunc("PUT /api/modules/welcome/settings", s.welcome.handleSettings)
	mux.HandleFunc("POST /api/modules/welcome/timer", s.welcome.handleTimer)
	mux.HandleFunc("POST /api/modules/welcome/obs/test", s.handleWelcomeOBSTest)
	mux.HandleFunc("POST /api/modules/welcome/obs/transition", s.welcome.handleTransition)
	mux.HandleFunc("POST /api/modules/welcome/upload/{kind}", s.welcome.handleUpload)
	mux.HandleFunc("GET /api/modules/welcome/assets/{assetID}", s.welcome.handleAsset)
	mux.HandleFunc("GET /api/modules/lights/state", s.lights.handleState)
	mux.HandleFunc("GET /api/modules/lights/settings", s.lights.handleSettings)
	mux.HandleFunc("PUT /api/modules/lights/settings", s.lights.handleSettings)
	mux.HandleFunc("POST /api/modules/lights/preview", s.lights.handlePreview)
	mux.HandleFunc("POST /api/modules/lights/beat", s.lights.handleBeat)
	mux.HandleFunc("GET /api/modules/lights/events", s.lights.handleEvents)
	mux.HandleFunc("POST /api/modules/lights/blackout", s.lights.handleBlackout)
	mux.HandleFunc("POST /internal/shutdown", s.handleShutdown)
}

func (s *workerService) handleWelcomeState(w http.ResponseWriter, _ *http.Request) {
	settings, timer, lastTransition := s.welcome.snapshot()
	s.mu.RLock()
	obs := s.obs
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"settings":       settings,
		"timer":          timer,
		"obs":            obs,
		"lastTransition": lastTransition,
	})
}

func (s *workerService) handleWelcomeOBSTest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	status := s.probe(ctx)
	s.mu.Lock()
	s.obs = status
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"connected": status.State == "connected", "connecting": false, "state": status.State, "label": status.Label, "detail": status.Detail})
}

func (s *workerService) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	total, err := s.store.Total(ctx)
	if err != nil {
		http.Error(w, "No se pudo consultar la base de eventos", http.StatusServiceUnavailable)
		return
	}
	schemaVersion, err := s.store.SchemaVersion(ctx)
	if err != nil {
		http.Error(w, "No se pudo consultar la versión del esquema", http.StatusServiceUnavailable)
		return
	}
	feeds := make([]feedStatus, 0, 2)
	s.mu.RLock()
	for _, id := range []string{"a", "b"} {
		feed := s.feeds[id]
		feeds = append(feeds, feed)
	}
	obs := s.obs
	lastSaved := s.lastSaved
	s.mu.RUnlock()
	for i := range feeds {
		count, countErr := s.store.Count(ctx, feeds[i].ID)
		if countErr != nil {
			http.Error(w, "No se pudo consultar la base de eventos", http.StatusServiceUnavailable)
			return
		}
		feeds[i].EventCount = count
		if feeds[i].LastEventAt == "" {
			feeds[i].LastEventAt, countErr = s.store.Latest(ctx, feeds[i].ID)
			if countErr != nil {
				http.Error(w, "No se pudo consultar la base de eventos", http.StatusServiceUnavailable)
				return
			}
		}
	}
	if lastSaved == "" && total > 0 {
		lastSaved, err = s.store.LatestEventTime(ctx)
		if err != nil {
			http.Error(w, "No se pudo consultar la base de eventos", http.StatusServiceUnavailable)
			return
		}
	}
	response := workerStatus{
		Storage: storageStatus{State: "connected", SchemaVersion: schemaVersion, EventCount: total, LastSaved: lastSaved},
		OBS:     obs,
		Feeds:   feeds,
	}
	response.Backend.State = "running"
	response.Backend.StartedAt = s.startedAt
	writeJSON(w, http.StatusOK, response)
}

func (s *workerService) handleEvents(w http.ResponseWriter, r *http.Request) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	events, err := s.store.Recent(ctx, 100)
	if err != nil {
		http.Error(w, "No se pudo leer el historial de comentarios", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *workerService) handleCaptureEvent(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		FeedID     string `json:"feedId"`
		SourceID   string `json:"sourceId"`
		ExternalID string `json:"externalId"`
		StreamID   string `json:"streamId"`
		Author     string `json:"author"`
		Message    string `json:"message"`
		EventAt    string `json:"eventAt"`
	}
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "El evento recibido no tiene un formato válido", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "El evento recibido contiene datos adicionales o inválidos", http.StatusBadRequest)
		return
	}
	input.FeedID = strings.ToLower(strings.TrimSpace(input.FeedID))
	input.SourceID = strings.ToLower(strings.TrimSpace(input.SourceID))
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.StreamID = strings.TrimSpace(input.StreamID)
	author := strings.TrimSpace(input.Author)
	message := strings.TrimSpace(input.Message)
	if (input.FeedID != "a" && input.FeedID != "b") || !validEventSourceID(input.SourceID) ||
		len(input.ExternalID) > 256 || input.StreamID == "" || len(input.StreamID) > 128 ||
		author == "" || len([]rune(author)) > 160 || message == "" || len([]rune(message)) > 4096 {
		http.Error(w, "El evento excede los límites o la cuenta/origen no es válido", http.StatusBadRequest)
		return
	}
	eventAt := time.Now().UTC()
	eventTimeForFingerprint := ""
	if input.EventAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, input.EventAt)
		if err != nil {
			http.Error(w, "La hora original del evento debe estar en formato RFC3339", http.StatusBadRequest)
			return
		}
		eventAt = parsed.UTC()
		eventTimeForFingerprint = eventAt.Format(time.RFC3339Nano)
	}
	eventTime := eventAt.Format(time.RFC3339Nano)
	dedupKey := "id:" + input.ExternalID
	if input.ExternalID == "" {
		fingerprint := sha256.Sum256([]byte(strings.Join([]string{
			input.SourceID, input.FeedID, input.StreamID, author, message, eventTimeForFingerprint,
		}, "\x00")))
		dedupKey = "hash:" + hex.EncodeToString(fingerprint[:])
	}

	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	stored, inserted, err := s.store.AddCaptured(ctx, capturedEvent{
		FeedID: input.FeedID, SourceID: input.SourceID, ExternalID: input.ExternalID,
		StreamID: input.StreamID, Author: author, Message: message, EventAt: eventTime,
		DedupKey: dedupKey,
	})
	if err != nil {
		http.Error(w, "No se pudo guardar el evento recibido", http.StatusServiceUnavailable)
		return
	}
	if !inserted {
		writeJSON(w, http.StatusOK, map[string]any{"state": "duplicate", "event": stored})
		return
	}
	s.mu.Lock()
	feed := s.feeds[input.FeedID]
	feed.LastEventAt = stored.CreatedAt
	feed.LastError = ""
	s.feeds[input.FeedID] = feed
	s.lastSaved = stored.CreatedAt
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"state": "saved", "event": stored})
}

func validEventSourceID(sourceID string) bool {
	if sourceID == "" || len(sourceID) > 64 {
		return false
	}
	for _, char := range sourceID {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == ':' {
			continue
		}
		return false
	}
	return true
}

func (s *workerService) handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "El identificador del comentario no es válido", http.StatusBadRequest)
		return
	}
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	deleted, err := s.store.Delete(ctx, id)
	if err != nil {
		http.Error(w, "No se pudo borrar el comentario guardado", http.StatusServiceUnavailable)
		return
	}
	if !deleted {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	s.lastSaved = ""
	for _, feed := range s.feeds {
		feed.LastEventAt = ""
		s.feeds[feed.ID] = feed
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"state": "deleted"})
}

func (s *workerService) handleDeleteAllEvents(w http.ResponseWriter, r *http.Request) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := s.store.DeleteAll(ctx); err != nil {
		http.Error(w, "No se pudo vaciar el historial de comentarios", http.StatusServiceUnavailable)
		return
	}
	s.mu.Lock()
	s.lastSaved = ""
	for _, feed := range s.feeds {
		feed.LastEventAt = ""
		s.feeds[feed.ID] = feed
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"state": "deleted"})
}

func (s *workerService) handleCaptureStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.ToLower(r.PathValue("id"))
	if id != "a" && id != "b" {
		http.Error(w, "La cuenta no es válida", http.StatusNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		CommentsPage bool  `json:"commentsPage"`
		VisibleRows  int   `json:"visibleRows"`
		NewComments  int   `json:"newComments"`
		LastDetected int64 `json:"lastDetected"`
	}
	if err := decoder.Decode(&input); err != nil || input.VisibleRows < 0 || input.VisibleRows > 500 || input.NewComments < 0 || input.NewComments > 1_000_000 || input.LastDetected < 0 {
		http.Error(w, "El estado del capturador no es válido", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "El estado del capturador contiene datos adicionales", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	feed := s.feeds[id]
	feed.CaptureAgentConnected = true
	feed.CommentsPage = input.CommentsPage
	feed.VisibleRows = input.VisibleRows
	feed.NewComments = input.NewComments
	feed.LastScanAt = now.Format(time.RFC3339Nano)
	feed.Connected = true
	feed.LastError = ""
	if input.CommentsPage {
		feed.State = "comments_page_ready"
	} else {
		feed.State = "facebook_open"
	}
	if input.LastDetected > 0 {
		feed.LastDetectedAt = time.UnixMilli(input.LastDetected).UTC().Format(time.RFC3339Nano)
	}
	s.feeds[id] = feed
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"state": feed.State})
}

func (s *workerService) handleSimulatedComment(w http.ResponseWriter, r *http.Request) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	var input struct {
		FeedID  string `json:"feedId"`
		Author  string `json:"author"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&input); err != nil {
		http.Error(w, "Datos de prueba inválidos", http.StatusBadRequest)
		return
	}
	input.FeedID = strings.ToLower(strings.TrimSpace(input.FeedID))
	if input.FeedID != "a" && input.FeedID != "b" {
		http.Error(w, "El feed de prueba debe ser A o B", http.StatusBadRequest)
		return
	}
	author := strings.TrimSpace(input.Author)
	message := strings.TrimSpace(input.Message)
	if author == "" || len([]rune(author)) > 80 || message == "" || len([]rune(message)) > 250 {
		http.Error(w, "El nombre o el comentario de prueba no es válido", http.StatusBadRequest)
		return
	}
	s.mu.RLock()
	feed := s.feeds[input.FeedID]
	s.mu.RUnlock()
	if !feed.Connected {
		http.Error(w, "El feed de prueba está desconectado", http.StatusConflict)
		return
	}
	event, err := s.store.Add(r.Context(), syntheticEvent{
		FeedID: input.FeedID, Author: author, Message: message,
	})
	if err != nil {
		http.Error(w, "No se pudo guardar el evento sintético", http.StatusServiceUnavailable)
		return
	}
	s.mu.Lock()
	feed = s.feeds[input.FeedID]
	feed.LastEventAt = event.CreatedAt
	feed.State = "simulado_conectado"
	feed.LastError = ""
	s.feeds[input.FeedID] = feed
	s.lastSaved = event.CreatedAt
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"event": event})
}

func (s *workerService) handleBackup(w http.ResponseWriter, r *http.Request) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	fileName, _, err := s.createLocalBackup(ctx)
	if err != nil {
		http.Error(w, "No se pudo crear y validar el respaldo local: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"fileName": fileName,
		"location": filepath.Join("backups", fileName),
		"state":    "created",
	})
}

func (s *workerService) handleBackups(w http.ResponseWriter, r *http.Request) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()

	directory, err := managedBackupDirectory(s.store.path, false)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusOK, map[string]any{"backups": []backupFileStatus{}})
		return
	}
	if err != nil {
		http.Error(w, "La carpeta de respaldos no es un directorio local válido", http.StatusInternalServerError)
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		http.Error(w, "No se pudo leer la carpeta local de respaldos", http.StatusInternalServerError)
		return
	}
	backups := make([]backupFileStatus, 0, len(entries))
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		name := entry.Name()
		if !isManagedBackupName(name) {
			continue
		}
		info, err := os.Lstat(filepath.Join(directory, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			http.Error(w, "No se pudo inspeccionar un respaldo local", http.StatusInternalServerError)
			return
		}
		if !info.Mode().IsRegular() {
			continue
		}
		backups = append(backups, backupFileStatus{
			Name: name, Size: info.Size(), Modified: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
}

func (s *workerService) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()

	name := r.PathValue("name")
	if !isManagedBackupName(name) {
		http.Error(w, "El nombre no corresponde a un respaldo administrado por Cortex Go", http.StatusBadRequest)
		return
	}
	directory, err := managedBackupDirectory(s.store.path, false)
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "La carpeta de respaldos no es un directorio local válido", http.StatusInternalServerError)
		return
	}
	path := filepath.Join(directory, name)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "No se pudo inspeccionar el respaldo", http.StatusInternalServerError)
		return
	}
	if !info.Mode().IsRegular() {
		http.Error(w, "El archivo no es un respaldo regular de Cortex Go", http.StatusBadRequest)
		return
	}
	if err := os.Remove(path); err != nil {
		http.Error(w, "No se pudo borrar el respaldo local", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": "deleted", "name": name})
}

func (s *workerService) handleDeleteAllBackups(w http.ResponseWriter, r *http.Request) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()

	directory, err := managedBackupDirectory(s.store.path, false)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusOK, map[string]any{"state": "deleted", "deletedCount": 0, "failedFiles": []string{}})
		return
	}
	if err != nil {
		http.Error(w, "La carpeta de respaldos no es un directorio local válido", http.StatusInternalServerError)
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		http.Error(w, "No se pudo leer la carpeta local de respaldos", http.StatusInternalServerError)
		return
	}
	deletedCount := 0
	failedFiles := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !isManagedBackupName(name) {
			continue
		}
		path := filepath.Join(directory, name)
		info, err := os.Lstat(path)
		if err == nil && info.Mode().IsRegular() {
			err = os.Remove(path)
		} else if err == nil {
			err = errors.New("el archivo no es regular")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			failedFiles = append(failedFiles, name)
			continue
		}
		if err == nil || errors.Is(err, os.ErrNotExist) {
			deletedCount++
		}
	}
	state := "deleted"
	if len(failedFiles) > 0 {
		state = "partial"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"state": state, "deletedCount": deletedCount, "failedFiles": failedFiles,
	})
}

func (s *workerService) handleRestore(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	r.Body = http.MaxBytesReader(w, r.Body, maxBackupUploadBytes)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		http.Error(w, "Archivo inválido o superior al límite de 128 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("backup")
	if err != nil {
		http.Error(w, "Seleccioná un archivo de respaldo SQLite", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if header.Size > maxBackupUploadBytes {
		http.Error(w, "El respaldo supera el límite de 128 MiB", http.StatusRequestEntityTooLarge)
		return
	}

	backupDirectory, err := managedBackupDirectory(s.store.path, true)
	if err != nil {
		http.Error(w, "No se pudo preparar la carpeta local de respaldos", http.StatusInternalServerError)
		return
	}
	temporary, err := os.CreateTemp(backupDirectory, ".restore-upload-*.sqlite")
	if err != nil {
		http.Error(w, "No se pudo preparar el archivo de restauración", http.StatusInternalServerError)
		return
	}
	temporaryPath := temporary.Name()
	defer removeSQLiteFiles(temporaryPath)
	bytesCopied, copyErr := io.Copy(temporary, io.LimitReader(file, maxBackupUploadBytes+1))
	closeErr := temporary.Close()
	if copyErr != nil || closeErr != nil {
		http.Error(w, "No se pudo leer el archivo de respaldo", http.StatusBadRequest)
		return
	}
	if bytesCopied > maxBackupUploadBytes {
		http.Error(w, "El respaldo supera el límite de 128 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	if bytesCopied == 0 {
		http.Error(w, "El archivo de respaldo está vacío", http.StatusBadRequest)
		return
	}
	if err := validateSQLiteSnapshot(ctx, temporaryPath); err != nil {
		http.Error(w, "El archivo no es un respaldo SQLite válido para Cortex Go", http.StatusUnprocessableEntity)
		return
	}

	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	fileName, _, err := s.createLocalBackup(ctx)
	if err != nil {
		http.Error(w, "No se pudo guardar la copia previa a la restauración: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	if err := s.store.Restore(ctx, temporaryPath); err != nil {
		http.Error(w, "No se pudo restaurar el respaldo; la copia previa se conservó en backups\\"+fileName+": "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if err := s.welcome.reload(); err != nil {
		http.Error(w, "La base se restauró, pero no se pudo recargar la configuración del módulo", http.StatusServiceUnavailable)
		return
	}

	s.mu.Lock()
	s.lastSaved = ""
	for _, id := range []string{"a", "b"} {
		feed := s.feeds[id]
		feed.LastEventAt = ""
		s.feeds[id] = feed
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{
		"state":             "restored",
		"restoredFile":      filepath.Base(header.Filename),
		"beforeRestoreFile": fileName,
	})
}

func (s *workerService) createLocalBackup(ctx context.Context) (fileName, path string, err error) {
	backupDirectory, err := managedBackupDirectory(s.store.path, true)
	if err != nil {
		return "", "", fmt.Errorf("preparar carpeta local: %w", err)
	}
	timestamp := time.Now().UTC().Format("20060102-150405-000000000")
	for sequence := 0; sequence < 1000; sequence++ {
		fileName = fmt.Sprintf("CortexGo-%s-%03d.sqlite", timestamp, sequence)
		path = filepath.Join(backupDirectory, fileName)
		err := s.store.Backup(ctx, path)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		return fileName, path, nil
	}
	return "", "", errors.New("no se pudo reservar un nombre único para el respaldo")
}

func isManagedBackupName(name string) bool {
	if strings.ContainsAny(name, `/\\`) || filepath.Base(name) != name || !strings.HasPrefix(name, "CortexGo-") || !strings.HasSuffix(name, ".sqlite") {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(name, "CortexGo-"), ".sqlite"), "-")
	if len(parts) != 4 || len(parts[0]) != 8 || len(parts[1]) != 6 || len(parts[2]) != 9 || len(parts[3]) != 3 {
		return false
	}
	for _, part := range parts {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return false
			}
		}
	}
	return true
}

func managedBackupDirectory(databasePath string, create bool) (string, error) {
	directory := filepath.Join(filepath.Dir(databasePath), "backups")
	if create {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return "", err
		}
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return directory, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("la carpeta de respaldos no debe ser un enlace ni otro tipo de archivo")
	}
	return directory, nil
}

func (s *workerService) handleFeedDisconnect(w http.ResponseWriter, r *http.Request) {
	s.updateFeedConnection(w, r, false)
}

func (s *workerService) handleFeedReconnect(w http.ResponseWriter, r *http.Request) {
	s.updateFeedConnection(w, r, true)
}

func (s *workerService) updateFeedConnection(w http.ResponseWriter, r *http.Request, connected bool) {
	id := strings.ToLower(r.PathValue("id"))
	s.mu.Lock()
	feed, ok := s.feeds[id]
	if !ok {
		s.mu.Unlock()
		http.Error(w, "Feed inexistente", http.StatusNotFound)
		return
	}
	feed.Connected = connected
	feed.LastError = ""
	if connected {
		feed.State = "simulado_conectado"
	} else {
		feed.State = "desconectado_simulado"
		feed.LastError = "Desconectado manualmente para la prueba"
	}
	s.feeds[id] = feed
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"feed": feed})
}

func (s *workerService) handleShutdown(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusAccepted, map[string]string{"state": "stopping"})
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.stopOnce.Do(func() { close(s.stop) })
	}()
}

func (s *workerService) watchOBS() {
	probeOnce := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		status := s.probe(ctx)
		status.LastCheckedAt = time.Now().UTC().Format(time.RFC3339)
		s.mu.Lock()
		s.obs = status
		s.mu.Unlock()
	}
	probeOnce()
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			probeOnce()
		case <-s.stop:
			return
		}
	}
}

func (s *workerService) watchParent(parentURL string) {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ticker.C:
			request, _ := http.NewRequest(http.MethodGet, strings.TrimRight(parentURL, "/")+"/healthz", nil)
			response, err := client.Do(request)
			if err == nil {
				_ = response.Body.Close()
				if response.Header.Get("X-Cortex-Go-Control-Plane") == "1" {
					failures = 0
					continue
				}
			}
			failures++
			if failures >= 3 {
				s.stopOnce.Do(func() { close(s.stop) })
				return
			}
		case <-s.stop:
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
