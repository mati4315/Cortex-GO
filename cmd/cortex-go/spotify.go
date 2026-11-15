package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	spotifyModuleID               = "spotify"
	spotifyScopes                 = "user-modify-playback-state user-read-playback-state user-read-currently-playing"
	spotifyCurrentRefreshInterval = 4 * time.Second
)

type spotifySettings struct {
	ClientID           string          `json:"clientId"`
	LiveQueueEnabled   bool            `json:"liveQueueEnabled"`
	RecentOrderMinutes int             `json:"recentOrderMinutes"`
	Training           spotifyTraining `json:"training"`
}

type spotifyAuthAttempt struct {
	State     string
	Verifier  string
	ExpiresAt time.Time
}

type spotifyCurrentTrack struct {
	URI        string          `json:"uri"`
	Name       string          `json:"name"`
	Artists    []spotifyArtist `json:"artists"`
	Album      spotifyAlbum    `json:"album"`
	DurationMS int64           `json:"duration_ms"`
	Type       string          `json:"type"`
	IsPlayable *bool           `json:"is_playable,omitempty"`
}

type spotifyArtist struct {
	Name string `json:"name"`
}

type spotifyAlbum struct {
	Name   string         `json:"name"`
	Images []spotifyImage `json:"images"`
}

type spotifyImage struct {
	URL string `json:"url"`
}

type spotifyPlayback struct {
	Device struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		IsActive bool   `json:"is_active"`
	} `json:"device"`
	IsPlaying  bool                 `json:"is_playing"`
	Timestamp  int64                `json:"timestamp"`
	ProgressMS int64                `json:"progress_ms"`
	Item       *spotifyCurrentTrack `json:"item"`
}

type spotifyService struct {
	store       *eventStore
	restoreMu   *sync.RWMutex
	client      *http.Client
	redirectURI string

	mu                 sync.RWMutex
	actionMu           sync.RWMutex
	currentRefreshMu   sync.Mutex
	settings           spotifySettings
	lastError          string
	current            *spotifyPlayback
	currentAt          string
	currentFetchAt     time.Time
	currentFetchFailed bool
	ruloGate           func() bool
	resultSink         func(context.Context, int64, string, string) error
	queueMu            sync.Mutex

	authMu       sync.Mutex
	auth         spotifyAuthAttempt
	tokenMu      sync.Mutex
	refreshToken string
	accessToken  string
	tokenExpiry  time.Time
}

func newSpotifyService(store *eventStore, parentURL string) (*spotifyService, error) {
	redirectURI, err := spotifyRedirectURI(parentURL)
	if err != nil {
		return nil, err
	}
	service := &spotifyService{store: store, client: &http.Client{Timeout: 8 * time.Second}, redirectURI: redirectURI, settings: defaultSpotifySettings()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var raw string
	err = store.db.QueryRowContext(ctx, `SELECT settings_json FROM module_settings WHERE module_id=?`, spotifyModuleID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		service.settings = defaultSpotifySettings()
		if err := service.persistSettings(ctx, service.settings); err != nil {
			return nil, fmt.Errorf("guardar la configuración inicial de Spotify: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("leer la configuración de Spotify: %w", err)
	} else if err := json.Unmarshal([]byte(raw), &service.settings); err != nil {
		return nil, fmt.Errorf("decodificar la configuración de Spotify: %w", err)
	}
	service.settings.Training = normalizeSpotifyTraining(service.settings.Training)
	if service.settings.RecentOrderMinutes == 0 {
		service.settings.RecentOrderMinutes = 10
		if err := service.persistSettings(ctx, service.settings); err != nil {
			return nil, fmt.Errorf("actualizar el valor inicial de vigencia Spotify: %w", err)
		}
	}
	if !validSpotifyClientID(service.settings.ClientID) && service.settings.ClientID != "" {
		return nil, errors.New("el Client ID guardado de Spotify no es válido")
	}
	if service.settings.RecentOrderMinutes < 1 || service.settings.RecentOrderMinutes > 30 {
		return nil, errors.New("la vigencia de pedidos Spotify guardada no es válida")
	}
	if spotifyTokenVaultAvailable() {
		token, err := loadSpotifyRefreshToken()
		if err == nil {
			service.refreshToken = strings.TrimSpace(token)
		} else if !errors.Is(err, os.ErrNotExist) {
			service.lastError = "No se pudo leer la autorización local de Spotify; vuelve a conectar la cuenta."
		}
	}
	return service, nil
}

func defaultSpotifySettings() spotifySettings {
	return spotifySettings{RecentOrderMinutes: 10, Training: defaultSpotifyTraining()}
}

func spotifyRedirectURI(parentURL string) (string, error) {
	if strings.TrimSpace(parentURL) == "" {
		parentURL = "http://" + defaultControlAddress
	}
	parsed, err := url.Parse(parentURL)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || !isLoopbackAddress(parsed.Host) {
		return "", errors.New("Spotify OAuth solo se habilita con un callback HTTP local")
	}
	if strings.EqualFold(parsed.Hostname(), "localhost") {
		if port := parsed.Port(); port != "" {
			parsed.Host = net.JoinHostPort("127.0.0.1", port)
		} else {
			parsed.Host = "127.0.0.1"
		}
	}
	parsed.Path = "/api/modules/spotify/callback"
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validSpotifyClientID(value string) bool {
	if len(value) < 20 || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func (s *spotifyService) persistSettings(ctx context.Context, settings spotifySettings) error {
	encoded, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO module_settings(module_id, settings_json, updated_at) VALUES(?, ?, ?)
		ON CONFLICT(module_id) DO UPDATE SET settings_json=excluded.settings_json, updated_at=excluded.updated_at`,
		spotifyModuleID, string(encoded), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *spotifyService) snapshot() (spotifySettings, bool, string, *spotifyPlayback, string) {
	s.mu.RLock()
	settings, lastError, current := s.settings, s.lastError, s.current
	s.mu.RUnlock()
	s.tokenMu.Lock()
	authenticated := s.refreshToken != "" && settings.ClientID != ""
	s.tokenMu.Unlock()
	if current != nil {
		copyCurrent := *current
		if current.Item != nil {
			item := *current.Item
			item.Artists = append([]spotifyArtist(nil), current.Item.Artists...)
			item.Album.Images = append([]spotifyImage(nil), current.Item.Album.Images...)
			copyCurrent.Item = &item
		}
		current = &copyCurrent
	}
	return settings, authenticated, s.redirectURI, current, lastError
}

func (s *spotifyService) registerRoutes(mux *http.ServeMux, restoreMu *sync.RWMutex) {
	registerRestoreGuarded(mux, restoreMu, "GET /api/modules/spotify/state", s.handleState)
	registerRestoreGuarded(mux, restoreMu, "GET /api/modules/spotify/settings", s.handleSettings)
	registerRestoreGuarded(mux, restoreMu, "PUT /api/modules/spotify/settings", s.handleSettings)
	registerRestoreGuarded(mux, restoreMu, "POST /api/modules/spotify/connect", s.handleConnect)
	registerRestoreGuarded(mux, restoreMu, "GET /api/modules/spotify/callback", s.handleCallback)
	registerRestoreGuarded(mux, restoreMu, "POST /api/modules/spotify/disconnect", s.handleDisconnect)
	registerRestoreGuarded(mux, restoreMu, "POST /api/modules/spotify/current", s.handleCurrent)
	registerRestoreGuarded(mux, restoreMu, "GET /api/modules/spotify/current", s.handleCurrentSnapshot)
	registerRestoreGuarded(mux, restoreMu, "GET /api/modules/spotify/operations", s.handleOperations)
	registerRestoreGuarded(mux, restoreMu, "POST /api/modules/spotify/search", s.handleSearch)
	registerRestoreGuarded(mux, restoreMu, "GET /api/modules/spotify/training", s.handleTraining)
	registerRestoreGuarded(mux, restoreMu, "PUT /api/modules/spotify/training", s.handleTraining)
	registerRestoreGuarded(mux, restoreMu, "POST /api/modules/spotify/training/reset", s.handleTrainingReset)
	registerRestoreGuarded(mux, restoreMu, "POST /api/modules/spotify/training/preview", s.handleTrainingPreview)
}

func (s *spotifyService) handleState(w http.ResponseWriter, _ *http.Request) {
	settings, authenticated, redirectURI, current, lastError := s.snapshot()
	s.mu.RLock()
	currentAt := s.currentAt
	s.mu.RUnlock()
	queueEnabled, queueMessage := s.queueReadiness()
	writeJSON(w, http.StatusOK, map[string]any{
		"settings": settings, "authenticated": authenticated, "tokenVaultAvailable": spotifyTokenVaultAvailable(),
		"redirectURI": redirectURI, "currentPlayback": current, "currentUpdatedAt": currentAt, "lastError": lastError,
		"queueEnabled": queueEnabled, "queueAuthorizationEnabled": settings.LiveQueueEnabled,
		"message": queueMessage,
	})
}

func (s *spotifyService) queueReadiness() (bool, string) {
	settings, authenticated, _, _, _ := s.snapshot()
	if !settings.LiveQueueEnabled {
		return false, "La cola automática está deshabilitada en Spotify."
	}
	if !authenticated {
		return false, "Conecta la cuenta Spotify antes de habilitar pedidos automáticos."
	}
	if s.ruloGate == nil {
		return false, "Rulo todavía no conectó el ejecutor Spotify."
	}
	if !s.ruloGate() {
		return false, "La habilidad Spotify espera que Rulo se reanude."
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var active int
	if err := s.store.db.QueryRowContext(ctx, `SELECT 1 FROM program_sessions WHERE status='active' LIMIT 1`).Scan(&active); err != nil {
		return false, "Inicia un programa para aceptar pedidos asociados a esa sesión."
	}
	return true, fmt.Sprintf("Spotify está habilitado para pedidos explícitos del programa actual; se recuperan solo solicitudes recientes durante %d minutos.", settings.RecentOrderMinutes)
}

func (s *spotifyService) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		settings, _, _, _, _ := s.snapshot()
		writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input spotifySettings
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "La configuración de Spotify no es válida", http.StatusBadRequest)
		return
	}
	input.ClientID = strings.TrimSpace(input.ClientID)
	if input.ClientID != "" && !validSpotifyClientID(input.ClientID) {
		http.Error(w, "El Client ID debe tener entre 20 y 64 caracteres alfanuméricos", http.StatusBadRequest)
		return
	}
	if input.RecentOrderMinutes == 0 {
		input.RecentOrderMinutes = 10
	}
	if input.RecentOrderMinutes < 1 || input.RecentOrderMinutes > 30 {
		http.Error(w, "La recuperación de pedidos recientes debe estar entre 1 y 30 minutos", http.StatusBadRequest)
		return
	}
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	s.mu.Lock()
	previous := s.settings
	s.mu.Unlock()
	if input.Training.Commands == nil && input.Training.MusicWords == nil && input.Training.RequestVerbs == nil {
		input.Training = cloneSpotifyTraining(previous.Training)
	} else {
		input.Training = normalizeSpotifyTraining(input.Training)
	}
	if previous.ClientID != input.ClientID {
		input.LiveQueueEnabled = false
	}
	if input.LiveQueueEnabled && (!spotifyTokenVaultAvailable() || input.ClientID == "" || !s.hasRefreshToken()) {
		http.Error(w, "Conecta primero la cuenta Spotify antes de habilitar la cola", http.StatusConflict)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.persistSettings(ctx, input); err != nil {
		http.Error(w, "No se pudo guardar la configuración de Spotify", http.StatusServiceUnavailable)
		return
	}
	if previous.ClientID != input.ClientID {
		if err := deleteSpotifyRefreshToken(); err != nil && spotifyTokenVaultAvailable() {
			if rollbackErr := s.persistSettings(ctx, previous); rollbackErr != nil {
				s.setLastError("No se pudo completar ni revertir el cambio de aplicación Spotify; revisa la configuración local.")
			}
			http.Error(w, "No se pudo retirar la autorización anterior; no se cambió el Client ID", http.StatusServiceUnavailable)
			return
		}
	}
	s.mu.Lock()
	s.settings = input
	if previous.ClientID != input.ClientID {
		s.current = nil
		s.lastError = ""
	}
	s.mu.Unlock()
	if previous.ClientID != input.ClientID {
		s.tokenMu.Lock()
		s.refreshToken, s.accessToken = "", ""
		s.tokenExpiry = time.Time{}
		s.tokenMu.Unlock()
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": input, "authenticated": input.ClientID != "" && previous.ClientID == input.ClientID && s.hasRefreshToken()})
}

func (s *spotifyService) handleConnect(w http.ResponseWriter, r *http.Request) {
	if !spotifyTokenVaultAvailable() {
		http.Error(w, "La autorización segura de Spotify solo está disponible en Windows", http.StatusNotImplemented)
		return
	}
	settings, _, _, _, _ := s.snapshot()
	if !validSpotifyClientID(settings.ClientID) {
		http.Error(w, "Guarda primero el Client ID de una aplicación Spotify creada por el operador", http.StatusConflict)
		return
	}
	stateBytes := make([]byte, 32)
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		http.Error(w, "No se pudo iniciar la autorización segura", http.StatusInternalServerError)
		return
	}
	if _, err := rand.Read(verifierBytes); err != nil {
		http.Error(w, "No se pudo iniciar la autorización segura", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	s.authMu.Lock()
	s.auth = spotifyAuthAttempt{State: state, Verifier: verifier, ExpiresAt: time.Now().Add(10 * time.Minute)}
	s.authMu.Unlock()
	query := url.Values{
		"client_id": {settings.ClientID}, "response_type": {"code"}, "redirect_uri": {s.redirectURI},
		"scope": {spotifyScopes}, "state": {state}, "code_challenge_method": {"S256"}, "code_challenge": {challenge},
		"show_dialog": {"true"},
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": "https://accounts.spotify.com/authorize?" + query.Encode(), "redirectURI": s.redirectURI, "scopes": strings.Fields(spotifyScopes)})
}

func (s *spotifyService) takeAuthAttempt(state string) (spotifyAuthAttempt, bool) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	attempt := s.auth
	if attempt.State == "" || attempt.State != state || time.Now().After(attempt.ExpiresAt) {
		if attempt.State != "" && time.Now().After(attempt.ExpiresAt) {
			s.auth = spotifyAuthAttempt{}
		}
		return spotifyAuthAttempt{}, false
	}
	s.auth = spotifyAuthAttempt{}
	return attempt, true
}

func (s *spotifyService) handleCallback(w http.ResponseWriter, r *http.Request) {
	attempt, ok := s.takeAuthAttempt(r.URL.Query().Get("state"))
	if !ok {
		s.setLastError("El intento de autorización venció o no coincide; vuelve a conectar Spotify.")
		http.Redirect(w, r, "/modules/spotify?auth=failed", http.StatusSeeOther)
		return
	}
	if r.URL.Query().Get("error") != "" {
		s.setLastError("La autorización de Spotify fue cancelada.")
		http.Redirect(w, r, "/modules/spotify?auth=failed", http.StatusSeeOther)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" || len(code) > 4096 {
		s.setLastError("Spotify no devolvió un código de autorización válido.")
		http.Redirect(w, r, "/modules/spotify?auth=failed", http.StatusSeeOther)
		return
	}
	settings, _, _, _, _ := s.snapshot()
	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {s.redirectURI},
		"client_id": {settings.ClientID}, "code_verifier": {attempt.Verifier},
	}
	response, err := s.postToken(r.Context(), form)
	if err != nil {
		s.setLastError("No se pudo completar la autorización con Spotify.")
		http.Redirect(w, r, "/modules/spotify?auth=failed", http.StatusSeeOther)
		return
	}
	defer response.Body.Close()
	var token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if response.StatusCode != http.StatusOK || decodeSpotifyResponse(response.Body, &token) != nil ||
		len(token.AccessToken) < 20 || len(token.AccessToken) > 8192 || len(token.RefreshToken) < 20 || len(token.RefreshToken) > 8192 ||
		!strings.EqualFold(token.TokenType, "Bearer") || token.ExpiresIn < 60 || token.ExpiresIn > 86400 || !hasSpotifyScopes(token.Scope) {
		s.setLastError("Spotify no devolvió una autorización completa con los permisos requeridos.")
		http.Redirect(w, r, "/modules/spotify?auth=failed", http.StatusSeeOther)
		return
	}
	if err := saveSpotifyRefreshToken(token.RefreshToken); err != nil {
		s.setLastError("No se pudo guardar la autorización cifrada local; conecta de nuevo después de revisar Windows.")
		http.Redirect(w, r, "/modules/spotify?auth=failed", http.StatusSeeOther)
		return
	}
	s.tokenMu.Lock()
	s.refreshToken, s.accessToken = token.RefreshToken, token.AccessToken
	s.tokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	s.tokenMu.Unlock()
	s.setLastError("")
	http.Redirect(w, r, "/modules/spotify?auth=ok", http.StatusSeeOther)
}

func hasSpotifyScopes(value string) bool {
	granted := make(map[string]bool)
	for _, scope := range strings.Fields(value) {
		granted[scope] = true
	}
	for _, required := range strings.Fields(spotifyScopes) {
		if !granted[required] {
			return false
		}
	}
	return true
}

func decodeSpotifyResponse(reader io.Reader, target any) error {
	data, err := io.ReadAll(io.LimitReader(reader, 64*1024+1))
	if err != nil || len(data) > 64*1024 {
		return errors.New("respuesta de Spotify demasiado grande")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("contenido adicional en respuesta de Spotify")
	}
	return nil
}

func (s *spotifyService) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	if err := deleteSpotifyRefreshToken(); err != nil && spotifyTokenVaultAvailable() {
		http.Error(w, "No se pudo retirar la autorización local de Spotify", http.StatusServiceUnavailable)
		return
	}
	settings := s.settingsSnapshot()
	settings.LiveQueueEnabled = false
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.persistSettings(ctx, settings); err != nil {
		s.setLastError("La autorización se retiró; no se pudo guardar el apagado de la cola. Revisa la configuración antes de volver a conectar.")
	}
	s.currentRefreshMu.Lock()
	defer s.currentRefreshMu.Unlock()
	s.tokenMu.Lock()
	s.refreshToken, s.accessToken = "", ""
	s.tokenExpiry = time.Time{}
	s.tokenMu.Unlock()
	s.mu.Lock()
	s.settings.LiveQueueEnabled = false
	s.current = nil
	s.currentAt = ""
	s.mu.Unlock()
	s.currentFetchAt = time.Time{}
	s.currentFetchFailed = false
	writeJSON(w, http.StatusOK, map[string]string{"state": "disconnected", "message": "Autorización local eliminada; si deseas revocarla también, hazlo en la cuenta Spotify."})
}

func (s *spotifyService) handleCurrent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	playback, updatedAt, _, _, err := s.refreshCurrentPlayback(ctx, true)
	if err != nil {
		s.setLastError(err.Error())
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"currentPlayback": playback, "currentUpdatedAt": updatedAt, "message": playbackMessage(playback)})
}

func (s *spotifyService) handleCurrentSnapshot(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	playback, updatedAt, connected, stale, err := s.refreshCurrentPlayback(ctx, false)
	message := playbackMessage(playback)
	if !connected {
		message = "Spotify no está conectado."
	} else if stale {
		message = "Mostrando la última lectura disponible de Spotify."
	} else if err != nil {
		if updatedAt != "" {
			message = "Spotify no responde ahora; se conserva la última lectura."
		} else {
			message = "No se pudo consultar el estado de Spotify."
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"currentPlayback": playback, "currentUpdatedAt": updatedAt, "connected": connected,
		"stale": stale, "message": message,
	})
}

func (s *spotifyService) refreshCurrentPlayback(ctx context.Context, force bool) (*spotifyPlayback, string, bool, bool, error) {
	s.currentRefreshMu.Lock()
	defer s.currentRefreshMu.Unlock()
	_, connected, _, current, _ := s.snapshot()
	s.mu.RLock()
	updatedAt := s.currentAt
	s.mu.RUnlock()
	if !connected {
		return current, updatedAt, false, false, errors.New("Spotify no está conectado")
	}
	if !force && !s.currentFetchAt.IsZero() && time.Since(s.currentFetchAt) < spotifyCurrentRefreshInterval {
		return current, updatedAt, true, s.currentFetchFailed, nil
	}
	playback, err := s.fetchCurrentPlayback(ctx)
	s.currentFetchAt = time.Now()
	if err != nil {
		s.currentFetchFailed = true
		s.setLastError(err.Error())
		_, _, _, current, _ = s.snapshot()
		s.mu.RLock()
		updatedAt = s.currentAt
		s.mu.RUnlock()
		return current, updatedAt, true, updatedAt != "", err
	}
	updatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.currentFetchFailed = false
	s.mu.Lock()
	s.current = playback
	s.currentAt = updatedAt
	s.lastError = ""
	s.mu.Unlock()
	return playback, updatedAt, true, false, nil
}

func playbackMessage(playback *spotifyPlayback) string {
	if playback == nil || playback.Item == nil {
		return "No hay una reproducción activa en Spotify."
	}
	return "Lectura confirmada desde Spotify."
}

func (s *spotifyService) fetchCurrentPlayback(ctx context.Context) (*spotifyPlayback, error) {
	data, status, err := s.spotifyGet(ctx, "https://api.spotify.com/v1/me/player")
	if err != nil {
		return nil, err
	}
	if status == http.StatusNoContent {
		return nil, nil
	}
	if status != http.StatusOK {
		return nil, spotifyStatusError(status)
	}
	var playback spotifyPlayback
	if err := decodeSpotifyResponse(strings.NewReader(string(data)), &playback); err != nil {
		return nil, errors.New("Spotify devolvió un estado de reproducción no válido")
	}
	if playback.Item != nil && len(playback.Item.Artists) > 8 {
		playback.Item.Artists = playback.Item.Artists[:8]
	}
	return &playback, nil
}

func (s *spotifyService) spotifyGet(ctx context.Context, endpoint string) ([]byte, int, error) {
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return nil, 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, 0, errors.New("no se pudo consultar Spotify")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		s.expireAccessToken()
		token, err = s.getAccessToken(ctx)
		if err != nil {
			return nil, 0, err
		}
		request, err = http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, 0, err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Accept", "application/json")
		response.Body.Close()
		response, err = s.client.Do(request)
		if err != nil {
			return nil, 0, errors.New("no se pudo consultar Spotify")
		}
		defer response.Body.Close()
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 512*1024+1))
	if err != nil || len(body) > 512*1024 {
		return nil, 0, errors.New("Spotify devolvió una respuesta no válida")
	}
	return body, response.StatusCode, nil
}

func (s *spotifyService) getAccessToken(ctx context.Context) (string, error) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if s.accessToken != "" && time.Until(s.tokenExpiry) > 30*time.Second {
		return s.accessToken, nil
	}
	if s.refreshToken == "" {
		return "", errors.New("conecta una cuenta Spotify desde el panel del módulo")
	}
	s.mu.RLock()
	clientID := s.settings.ClientID
	s.mu.RUnlock()
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {s.refreshToken}, "client_id": {clientID}}
	response, err := s.postToken(ctx, form)
	if err != nil {
		return "", errors.New("no se pudo renovar la sesión de Spotify")
	}
	defer response.Body.Close()
	var token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if response.StatusCode != http.StatusOK || decodeSpotifyResponse(response.Body, &token) != nil ||
		len(token.AccessToken) < 20 || len(token.AccessToken) > 8192 || !strings.EqualFold(token.TokenType, "Bearer") || token.ExpiresIn < 60 || token.ExpiresIn > 86400 {
		return "", errors.New("Spotify rechazó la renovación; vuelve a conectar la cuenta")
	}
	if token.RefreshToken != "" && token.RefreshToken != s.refreshToken {
		if err := saveSpotifyRefreshToken(token.RefreshToken); err != nil {
			return "", errors.New("no se pudo actualizar el token cifrado de Spotify")
		}
		s.refreshToken = token.RefreshToken
	}
	s.accessToken = token.AccessToken
	s.tokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return s.accessToken, nil
}

func (s *spotifyService) postToken(ctx context.Context, form url.Values) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	return s.client.Do(request)
}

func (s *spotifyService) expireAccessToken() {
	s.tokenMu.Lock()
	s.accessToken = ""
	s.tokenExpiry = time.Time{}
	s.tokenMu.Unlock()
}

func (s *spotifyService) hasRefreshToken() bool {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	return s.refreshToken != ""
}

func spotifyStatusError(status int) error {
	switch status {
	case http.StatusUnauthorized:
		return errors.New("Spotify no autorizó la consulta; vuelve a conectar la cuenta")
	case http.StatusForbidden:
		return errors.New("Spotify no permite la consulta con esta cuenta o estos permisos")
	case http.StatusNotFound, http.StatusNoContent:
		return errors.New("Spotify no tiene una reproducción activa")
	case http.StatusTooManyRequests:
		return errors.New("Spotify limitó las consultas; espera antes de volver a actualizar")
	default:
		return fmt.Errorf("Spotify respondió con un error HTTP %d", status)
	}
}

func (s *spotifyService) setLastError(message string) {
	s.mu.Lock()
	s.lastError = message
	s.mu.Unlock()
}

func (s *spotifyService) reloadAfterRestore(ctx context.Context) bool {
	settings := defaultSpotifySettings()
	var raw string
	if err := s.store.db.QueryRowContext(ctx, `SELECT settings_json FROM module_settings WHERE module_id=?`, spotifyModuleID).Scan(&raw); err == nil {
		if json.Unmarshal([]byte(raw), &settings) != nil {
			settings = defaultSpotifySettings()
		}
	}
	if settings.RecentOrderMinutes < 1 || settings.RecentOrderMinutes > 30 {
		settings.RecentOrderMinutes = 10
	}
	settings.LiveQueueEnabled = false
	s.mu.Lock()
	s.settings = settings
	s.current = nil
	s.currentAt = ""
	s.lastError = "La base se restauró; vuelve a conectar Spotify antes de usar pedidos automáticos."
	s.mu.Unlock()
	s.tokenMu.Lock()
	s.refreshToken, s.accessToken = "", ""
	s.tokenExpiry = time.Time{}
	s.tokenMu.Unlock()
	s.authMu.Lock()
	s.auth = spotifyAuthAttempt{}
	s.authMu.Unlock()
	if !spotifyTokenVaultAvailable() {
		return true
	}
	err := deleteSpotifyRefreshToken()
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	if err != nil {
		s.setLastError("La cola quedó deshabilitada, pero no se pudo retirar la autorización Spotify local; desconéctala desde el panel.")
		return false
	}
	return true
}

func (s *spotifyService) readiness() moduleReadinessStatus {
	settings, authenticated, _, _, lastError := s.snapshot()
	if settings.ClientID == "" {
		return moduleReadinessStatus{State: "attention", Detail: "Requiere un Client ID nuevo del operador; no se importaron credenciales antiguas"}
	}
	if !spotifyTokenVaultAvailable() {
		return moduleReadinessStatus{State: "attention", Detail: "El almacén de autorización seguro requiere Windows"}
	}
	if !authenticated {
		return moduleReadinessStatus{State: "attention", Detail: "Falta autorizar la cuenta Spotify desde el panel"}
	}
	if lastError != "" {
		return moduleReadinessStatus{State: "attention", Detail: lastError}
	}
	if !settings.LiveQueueEnabled {
		return moduleReadinessStatus{State: "ready", Detail: "Cuenta autorizada; la cola automática está deshabilitada"}
	}
	queueReady, queueMessage := s.queueReadiness()
	if !queueReady {
		return moduleReadinessStatus{State: "attention", Detail: queueMessage}
	}
	return moduleReadinessStatus{State: "ready", Detail: queueMessage}
}
