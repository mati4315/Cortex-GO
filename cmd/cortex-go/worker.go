package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type feedStatus struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Connected   bool   `json:"connected"`
	LastEventAt string `json:"lastEventAt,omitempty"`
	EventCount  int    `json:"eventCount"`
	LastError   string `json:"lastError,omitempty"`
}

type storageStatus struct {
	State      string `json:"state"`
	EventCount int    `json:"eventCount"`
	LastSaved  string `json:"lastSaved,omitempty"`
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
	startedAt string
	probe     func(context.Context) obsStatus

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
			"a": {ID: "a", Name: "Feed de prueba A", State: "simulado_conectado", Connected: true},
			"b": {ID: "b", Name: "Feed de prueba B", State: "simulado_conectado", Connected: true},
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
	mux.HandleFunc("POST /api/simulate/comment", s.handleSimulatedComment)
	mux.HandleFunc("POST /api/feeds/{id}/disconnect", s.handleFeedDisconnect)
	mux.HandleFunc("POST /api/feeds/{id}/reconnect", s.handleFeedReconnect)
	mux.HandleFunc("POST /internal/shutdown", s.handleShutdown)
}

func (s *workerService) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	total, err := s.store.Total(ctx)
	if err != nil {
		http.Error(w, "No se pudo consultar la base sintética", http.StatusServiceUnavailable)
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
			http.Error(w, "No se pudo consultar la base sintética", http.StatusServiceUnavailable)
			return
		}
		feeds[i].EventCount = count
		if feeds[i].LastEventAt == "" {
			feeds[i].LastEventAt, countErr = s.store.Latest(ctx, feeds[i].ID)
			if countErr != nil {
				http.Error(w, "No se pudo consultar la base sintética", http.StatusServiceUnavailable)
				return
			}
		}
	}
	if lastSaved == "" && total > 0 {
		lastSaved, err = s.store.LatestEventTime(ctx)
		if err != nil {
			http.Error(w, "No se pudo consultar la base sintética", http.StatusServiceUnavailable)
			return
		}
	}
	response := workerStatus{
		Storage: storageStatus{State: "connected", EventCount: total, LastSaved: lastSaved},
		OBS:     obs,
		Feeds:   feeds,
	}
	response.Backend.State = "running"
	response.Backend.StartedAt = s.startedAt
	writeJSON(w, http.StatusOK, response)
}

func (s *workerService) handleEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	events, err := s.store.Recent(ctx, 100)
	if err != nil {
		http.Error(w, "No se pudo leer el historial sintético", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *workerService) handleSimulatedComment(w http.ResponseWriter, r *http.Request) {
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
