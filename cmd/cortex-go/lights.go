package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const lightsModuleID = "lights"

var lightsColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var lightsClientIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

type lightsConfig struct {
	Preset           string `json:"preset"`
	Speed            int    `json:"speed"`
	Intensity        int    `json:"intensity"`
	Fog              int    `json:"fog"`
	ColorPalette     string `json:"colorPalette"`
	CustomColor1     string `json:"customColor1"`
	CustomColor2     string `json:"customColor2"`
	DiscoBall        bool   `json:"discoBall"`
	DiscoBallSpeed   int    `json:"discoBallSpeed"`
	DiscoBallDots    bool   `json:"discoBallDots"`
	Lasers           bool   `json:"lasers"`
	LaserCount       int    `json:"laserCount"`
	LaserColorMode   string `json:"laserColorMode"`
	LaserPattern     string `json:"laserPattern"`
	LaserSpeed       int    `json:"laserSpeed"`
	Spotlights       bool   `json:"spotlights"`
	SpotlightCount   int    `json:"spotlightCount"`
	SpotlightPattern string `json:"spotlightPattern"`
	SpotlightSpeed   int    `json:"spotlightSpeed"`
	SpotlightGobo    string `json:"spotlightGobo"`
	Strobe           bool   `json:"strobe"`
	StrobeSpeed      int    `json:"strobeSpeed"`
	StrobeColor      string `json:"strobeColor"`
	StrobeMode       string `json:"strobeMode"`
	Wash             bool   `json:"wash"`
	WashSpeed        int    `json:"washSpeed"`
	AudioReactive    bool   `json:"audioReactive"`
	AudioSensitivity int    `json:"audioSensitivity"`
	BPM              int    `json:"bpm"`
	VirtualBeat      bool   `json:"virtualBeat"`
}

type lightsClient struct {
	id       string
	typeName string
	events   chan lightsEvent
}

type lightsEvent struct {
	name    string
	payload json.RawMessage
}

type lightsConnectionStatus struct {
	Overlays   int `json:"overlays"`
	Dashboards int `json:"dashboards"`
}

type lightsService struct {
	store   *eventStore
	mu      sync.Mutex
	config  lightsConfig
	clients map[string]*lightsClient
}

func defaultLightsConfig() lightsConfig {
	return lightsConfig{
		Preset: "automatic", Speed: 50, Intensity: 80, Fog: 40, ColorPalette: "rainbow",
		CustomColor1: "#ff0055", CustomColor2: "#00ffcc", DiscoBall: true, DiscoBallSpeed: 40,
		DiscoBallDots: true, Lasers: true, LaserCount: 4, LaserColorMode: "rainbow",
		LaserPattern: "bounce", LaserSpeed: 50, Spotlights: true, SpotlightCount: 4,
		SpotlightPattern: "sweep", SpotlightSpeed: 40, SpotlightGobo: "cone", Strobe: false,
		StrobeSpeed: 3, StrobeColor: "#ffffff", StrobeMode: "freq", Wash: true, WashSpeed: 50,
		AudioReactive: false, AudioSensitivity: 50, BPM: 128, VirtualBeat: true,
	}
}

func newLightsService(store *eventStore) (*lightsService, error) {
	service := &lightsService{store: store, config: defaultLightsConfig(), clients: make(map[string]*lightsClient)}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var settingsJSON string
	err := store.db.QueryRowContext(ctx, `SELECT settings_json FROM module_settings WHERE module_id = ?`, lightsModuleID).Scan(&settingsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		if err := service.persist(ctx, service.config); err != nil {
			return nil, fmt.Errorf("guardar la configuración inicial de Luces: %w", err)
		}
		return service, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leer la configuración de Luces: %w", err)
	}
	if err := json.Unmarshal([]byte(settingsJSON), &service.config); err != nil {
		return nil, fmt.Errorf("decodificar la configuración de Luces: %w", err)
	}
	if service.config.StrobeSpeed > 3 {
		service.config.StrobeSpeed = 3
		if err := service.persist(ctx, service.config); err != nil {
			return nil, fmt.Errorf("actualizar el límite de estrobo de Luces: %w", err)
		}
	}
	if err := validateLightsConfig(service.config); err != nil {
		return nil, fmt.Errorf("la configuración guardada de Luces no es válida: %w", err)
	}
	return service, nil
}

func (s *lightsService) persist(ctx context.Context, config lightsConfig) error {
	content, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO module_settings(module_id, settings_json, updated_at)
		VALUES(?, ?, ?) ON CONFLICT(module_id) DO UPDATE SET settings_json = excluded.settings_json, updated_at = excluded.updated_at`,
		lightsModuleID, string(content), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *lightsService) snapshot() lightsConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

func (s *lightsService) handleState(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	config := s.config
	connections := s.connectionStatusLocked()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"config": config, "connections": connections})
}

func (s *lightsService) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "config": s.snapshot()})
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	var request map[string]json.RawMessage
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "La configuración de Luces no es un JSON válido", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "La configuración de Luces contiene datos adicionales", http.StatusBadRequest)
		return
	}
	patch, err := lightsConfigPatch(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	config, err := s.updateConfig(r.Context(), patch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.broadcast("configUpdate", config, "")
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "config": config})
}

func lightsConfigPatch(request map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	if len(request) == 0 {
		return nil, errors.New("la configuración está vacía")
	}
	if changes, ok := request["changes"]; ok {
		if len(request) != 1 {
			return nil, errors.New("envía cambios dentro del campo changes")
		}
		var patch map[string]json.RawMessage
		if err := json.Unmarshal(changes, &patch); err != nil || patch == nil {
			return nil, errors.New("el campo changes no es válido")
		}
		request = patch
	}
	allowed := map[string]bool{
		"preset": true, "speed": true, "intensity": true, "fog": true, "colorPalette": true,
		"customColor1": true, "customColor2": true, "discoBall": true, "discoBallSpeed": true,
		"discoBallDots": true, "lasers": true, "laserCount": true, "laserColorMode": true,
		"laserPattern": true, "laserSpeed": true, "spotlights": true, "spotlightCount": true,
		"spotlightPattern": true, "spotlightSpeed": true, "spotlightGobo": true, "strobe": true,
		"strobeSpeed": true, "strobeColor": true, "strobeMode": true, "wash": true,
		"washSpeed": true, "audioReactive": true, "audioSensitivity": true, "bpm": true, "virtualBeat": true,
	}
	for key := range request {
		if !allowed[key] {
			return nil, fmt.Errorf("el campo %q no pertenece al módulo Luces", key)
		}
	}
	return request, nil
}

func (s *lightsService) updateConfig(ctx context.Context, patch map[string]json.RawMessage) (lightsConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	encoded, err := json.Marshal(s.config)
	if err != nil {
		return lightsConfig{}, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &merged); err != nil {
		return lightsConfig{}, err
	}
	for key, value := range patch {
		merged[key] = value
	}
	encoded, err = json.Marshal(merged)
	if err != nil {
		return lightsConfig{}, err
	}
	var next lightsConfig
	if err := json.Unmarshal(encoded, &next); err != nil {
		return lightsConfig{}, errors.New("uno o más valores tienen un tipo incorrecto")
	}
	if err := validateLightsConfig(next); err != nil {
		return lightsConfig{}, err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := s.persist(writeCtx, next); err != nil {
		return lightsConfig{}, fmt.Errorf("no se pudo guardar la configuración de Luces: %w", err)
	}
	s.config = next
	return next, nil
}

func validateLightsConfig(config lightsConfig) error {
	if !oneOf(config.Preset, "automatic", "electronic", "retro_disco", "chill_out", "psychedelic", "laser_show") {
		return errors.New("preset de iluminación no válido")
	}
	for name, value := range map[string]int{"speed": config.Speed, "intensity": config.Intensity, "fog": config.Fog,
		"discoBallSpeed": config.DiscoBallSpeed, "laserSpeed": config.LaserSpeed, "spotlightSpeed": config.SpotlightSpeed,
		"washSpeed": config.WashSpeed} {
		minimum := 5
		if name == "intensity" {
			minimum = 10
		} else if name == "fog" {
			minimum = 0
		}
		if value < minimum || value > 100 {
			return fmt.Errorf("%s debe estar entre %d y 100", name, minimum)
		}
	}
	if config.AudioSensitivity < 10 || config.AudioSensitivity > 100 {
		return errors.New("audioSensitivity debe estar entre 10 y 100")
	}
	if config.StrobeSpeed < 2 || config.StrobeSpeed > 3 {
		return errors.New("strobeSpeed debe estar entre 2 y 3 Hz")
	}
	if config.BPM < 60 || config.BPM > 200 {
		return errors.New("bpm debe estar entre 60 y 200")
	}
	for name, value := range map[string]int{"laserCount": config.LaserCount, "spotlightCount": config.SpotlightCount} {
		if value != 2 && value != 4 && value != 6 && value != 8 {
			return fmt.Errorf("%s debe ser 2, 4, 6 u 8", name)
		}
	}
	if !oneOf(config.ColorPalette, "rainbow", "neon", "warm", "cold", "laser", "custom") {
		return errors.New("paleta de color no válida")
	}
	if !oneOf(config.LaserColorMode, "rainbow", "laser", "cold") ||
		!oneOf(config.LaserPattern, "bounce", "wave", "cross", "circle") ||
		!oneOf(config.SpotlightPattern, "sweep", "circles", "sync", "dance") ||
		!oneOf(config.SpotlightGobo, "cone", "dots") || !oneOf(config.StrobeMode, "freq", "beat") {
		return errors.New("uno o más patrones de iluminación no son válidos")
	}
	for name, color := range map[string]string{"customColor1": config.CustomColor1, "customColor2": config.CustomColor2, "strobeColor": config.StrobeColor} {
		if !lightsColorPattern.MatchString(color) {
			return fmt.Errorf("%s debe usar formato hexadecimal #RRGGBB", name)
		}
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func (s *lightsService) handleBeat(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<10)
	var request struct {
		ClientID string `json:"clientId"`
		Origin   string `json:"origin"`
		Time     int64  `json:"time"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "El pulso de ritmo no es válido", http.StatusBadRequest)
		return
	}
	if !lightsClientIDPattern.MatchString(request.ClientID) {
		http.Error(w, "El cliente de ritmo no es válido", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	client, connected := s.clients[request.ClientID]
	s.mu.Unlock()
	if !connected {
		http.Error(w, "La página que envió el pulso ya no está conectada", http.StatusConflict)
		return
	}
	if request.Origin != "dashboard" && request.Origin != "overlay" {
		request.Origin = client.typeName
	}
	if request.Time <= 0 {
		request.Time = time.Now().UnixMilli()
	}
	s.broadcast("beatTrigger", map[string]any{"origin": request.Origin, "time": request.Time}, request.ClientID)
	writeJSON(w, http.StatusAccepted, map[string]bool{"success": true})
}

func (s *lightsService) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		http.Error(w, "Se requiere text/event-stream", http.StatusNotAcceptable)
		return
	}
	typeName := r.URL.Query().Get("type")
	if typeName != "dashboard" && typeName != "overlay" {
		http.Error(w, "El tipo de cliente debe ser dashboard u overlay", http.StatusBadRequest)
		return
	}
	clientID := r.URL.Query().Get("clientId")
	if !lightsClientIDPattern.MatchString(clientID) {
		http.Error(w, "El identificador de cliente no es válido", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "La conexión no admite eventos en tiempo real", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")

	client := &lightsClient{id: clientID, typeName: typeName, events: make(chan lightsEvent, 32)}
	s.mu.Lock()
	s.clients[clientID] = client
	config := s.config
	connections := s.connectionStatusLocked()
	s.mu.Unlock()
	defer s.removeClient(clientID, client)

	w.WriteHeader(http.StatusOK)
	if writeLightsEvent(w, flusher, "configUpdate", config) != nil || writeLightsEvent(w, flusher, "connectionStatus", connections) != nil {
		return
	}
	s.broadcast("connectionStatus", connections, clientID)
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-client.events:
			if err := writeRawLightsEvent(w, flusher, event); err != nil {
				return
			}
		case <-ticker.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *lightsService) removeClient(clientID string, client *lightsClient) {
	s.mu.Lock()
	if s.clients[clientID] != client {
		s.mu.Unlock()
		return
	}
	delete(s.clients, clientID)
	connections := s.connectionStatusLocked()
	s.mu.Unlock()
	s.broadcast("connectionStatus", connections, "")
}

func (s *lightsService) connectionStatusLocked() lightsConnectionStatus {
	var status lightsConnectionStatus
	for _, client := range s.clients {
		switch client.typeName {
		case "overlay":
			status.Overlays++
		case "dashboard":
			status.Dashboards++
		}
	}
	return status
}

func (s *lightsService) broadcast(name string, payload any, exceptClientID string) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	event := lightsEvent{name: name, payload: data}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, client := range s.clients {
		if id == exceptClientID {
			continue
		}
		select {
		case client.events <- event:
		default:
			select {
			case <-client.events:
			default:
			}
			select {
			case client.events <- event:
			default:
			}
		}
	}
}

func writeLightsEvent(w http.ResponseWriter, flusher http.Flusher, name string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return writeRawLightsEvent(w, flusher, lightsEvent{name: name, payload: data})
}

func writeRawLightsEvent(w http.ResponseWriter, flusher http.Flusher, event lightsEvent) error {
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.name, event.payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
