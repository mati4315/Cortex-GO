package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const facebookOverlayQueueSize = 256

type facebookOverlayClient struct {
	id       string
	typeName string
	events   chan facebookOverlayMessage
}

type facebookOverlayMessage struct {
	name    string
	id      int64
	payload any
}

type facebookFeaturedMessage struct {
	Event      syntheticEvent `json:"event"`
	FeaturedAt string         `json:"featuredAt"`
	ArchiveID  int64          `json:"archiveId,omitempty"`
}

type facebookOverlayService struct {
	store     *eventStore
	storageMu *sync.Mutex
	stop      <-chan struct{}
	mu        sync.Mutex
	clients   map[string]*facebookOverlayClient
	featured  *facebookFeaturedMessage
}

func newFacebookOverlayService(store *eventStore, storageMu *sync.Mutex, stop <-chan struct{}) *facebookOverlayService {
	return &facebookOverlayService{
		store:     store,
		storageMu: storageMu,
		stop:      stop,
		clients:   make(map[string]*facebookOverlayClient),
	}
}

func (s *facebookOverlayService) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("type") != "overlay" {
		http.Error(w, "El tipo de cliente debe ser overlay", http.StatusBadRequest)
		return
	}
	s.handleEventStream(w, r, "overlay")
}

func (s *facebookOverlayService) handleFeaturedEvents(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("type") != "featured" {
		http.Error(w, "El tipo de cliente debe ser featured", http.StatusBadRequest)
		return
	}
	s.handleEventStream(w, r, "featured")
}

func (s *facebookOverlayService) handleEventStream(w http.ResponseWriter, r *http.Request, typeName string) {
	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		http.Error(w, "Se requiere text/event-stream", http.StatusNotAcceptable)
		return
	}
	clientID := r.URL.Query().Get("clientId")
	if !validFacebookOverlayClientID(clientID) {
		http.Error(w, "El identificador de la fuente no es válido", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "La conexión no admite eventos en tiempo real", http.StatusInternalServerError)
		return
	}

	lastEventIDHeader := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	hasLastEventID := typeName == "overlay" && lastEventIDHeader != ""
	var lastEventID int64
	if hasLastEventID {
		parsed, err := strconv.ParseInt(lastEventIDHeader, 10, 64)
		if err != nil || parsed < 0 {
			http.Error(w, "El cursor de reconexión no es válido", http.StatusBadRequest)
			return
		}
		lastEventID = parsed
	}

	client := &facebookOverlayClient{
		id:       clientID,
		typeName: typeName,
		events:   make(chan facebookOverlayMessage, facebookOverlayQueueSize),
	}
	var initialCursor int64
	if typeName == "overlay" {
		s.storageMu.Lock()
	}
	s.mu.Lock()
	if typeName == "overlay" && !hasLastEventID {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		cursor, err := s.store.MaxEventID(ctx)
		cancel()
		if err != nil {
			s.mu.Unlock()
			s.storageMu.Unlock()
			http.Error(w, "No se pudo preparar la conexión del overlay", http.StatusServiceUnavailable)
			return
		}
		initialCursor = cursor
	} else if hasLastEventID {
		initialCursor = lastEventID
	}
	if previous := s.clients[clientID]; previous != nil {
		delete(s.clients, clientID)
		close(previous.events)
	}
	s.clients[clientID] = client
	var featuredSnapshot *facebookFeaturedMessage
	if s.featured != nil {
		copy := *s.featured
		featuredSnapshot = &copy
	}
	s.mu.Unlock()
	if typeName == "overlay" {
		s.storageMu.Unlock()
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	includeReadyID := typeName == "overlay" && !hasLastEventID
	if err := writeFacebookOverlaySSE(w, flusher, "ready", initialCursor, includeReadyID, map[string]any{
		"cursor":       initialCursor,
		"reconnecting": hasLastEventID,
	}); err != nil {
		s.removeClient(client)
		return
	}
	if featuredSnapshot != nil {
		if err := writeFacebookOverlaySSE(w, flusher, "featured", 0, false, featuredSnapshot); err != nil {
			s.removeClient(client)
			return
		}
	}

	lastSentID := initialCursor
	if typeName == "overlay" && hasLastEventID {
		for {
			select {
			case <-r.Context().Done():
				s.removeClient(client)
				return
			case <-s.stop:
				s.removeClient(client)
				return
			default:
			}
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			s.storageMu.Lock()
			batch, err := s.store.After(ctx, lastSentID, 250)
			s.storageMu.Unlock()
			cancel()
			if err != nil {
				_ = writeFacebookOverlaySSE(w, flusher, "streamError", 0, false, map[string]string{"message": "No se pudo recuperar la conexión del overlay"})
				s.removeClient(client)
				return
			}
			for _, event := range batch {
				select {
				case <-r.Context().Done():
					s.removeClient(client)
					return
				case <-s.stop:
					s.removeClient(client)
					return
				default:
				}
				if err := writeFacebookOverlaySSE(w, flusher, "comment", event.ID, true, event); err != nil {
					s.removeClient(client)
					return
				}
				lastSentID = event.ID
			}
			if len(batch) < 250 {
				break
			}
		}
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	defer s.removeClient(client)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.stop:
			return
		case message, open := <-client.events:
			if !open {
				return
			}
			if message.name == "comment" {
				if typeName != "overlay" || message.id <= lastSentID {
					continue
				}
			}
			if err := writeFacebookOverlaySSE(w, flusher, message.name, message.id, message.name == "comment", message.payload); err != nil {
				return
			}
			if message.name == "comment" {
				lastSentID = message.id
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *facebookOverlayService) broadcast(event syntheticEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.broadcastLocked("overlay", facebookOverlayMessage{name: "comment", id: event.ID, payload: event})
}

func (s *facebookOverlayService) broadcastRuloResponse(update ruloResponseUpdate) {
	eventName := "ruloResponse"
	if update.Response.Version > 1 {
		eventName = "ruloResponseUpdate"
	}
	s.mu.Lock()
	s.broadcastLocked("overlay", facebookOverlayMessage{name: eventName, payload: update})
	s.mu.Unlock()
}

func (s *facebookOverlayService) broadcastRuloMute(muted bool, cursor int64) {
	s.mu.Lock()
	s.broadcastLocked("overlay", facebookOverlayMessage{name: "ruloPresentation", payload: map[string]any{"muted": muted, "cursor": cursor}})
	s.mu.Unlock()
}

func (s *facebookOverlayService) broadcastRuloOverlayConfig(state ruloUnifiedOverlayConfigState) {
	s.mu.Lock()
	s.broadcastLocked("overlay", facebookOverlayMessage{name: "ruloOverlayConfig", payload: state})
	s.mu.Unlock()
}

func (s *facebookOverlayService) setFeatured(event syntheticEvent, archiveID int64) facebookFeaturedMessage {
	featured := facebookFeaturedMessage{Event: event, FeaturedAt: time.Now().UTC().Format(time.RFC3339Nano), ArchiveID: archiveID}
	s.mu.Lock()
	s.featured = &featured
	s.broadcastLocked("", facebookOverlayMessage{name: "featured", payload: featured})
	s.mu.Unlock()
	return featured
}

func (s *facebookOverlayService) broadcastLocked(typeName string, message facebookOverlayMessage) {
	for id, client := range s.clients {
		if typeName != "" && client.typeName != typeName {
			continue
		}
		select {
		case client.events <- message:
		default:
			delete(s.clients, id)
			close(client.events)
		}
	}
}

func (s *facebookOverlayService) removeClient(client *facebookOverlayClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clients[client.id] == client {
		delete(s.clients, client.id)
		close(client.events)
	}
}

func validFacebookOverlayClientID(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func writeFacebookOverlaySSE(w http.ResponseWriter, flusher http.Flusher, name string, id int64, includeID bool, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if includeID {
		if _, err := fmt.Fprintf(w, "id: %d\n", id); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
