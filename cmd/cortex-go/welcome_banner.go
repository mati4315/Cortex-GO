package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	welcomeModuleID       = "bienvenidos-banner"
	maxWelcomeSettings    = 64 << 10
	maxWelcomeImageBytes  = 20 << 20
	maxWelcomeVideoBytes  = 30 << 20
	maxWelcomeAudioBytes  = 16 << 20
	welcomeTimerMaxSecond = 99*60 + 59
)

var (
	welcomeColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	welcomeAssetPattern = regexp.MustCompile(`^/api/modules/welcome/assets/([a-f0-9]{32})$`)
)

type welcomeSocial struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Handle   string `json:"handle"`
	Visible  bool   `json:"visible"`
}

type welcomeBannerItem struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Label    string `json:"label"`
	Handle   string `json:"handle"`
	Visible  bool   `json:"visible"`
}

type welcomeBannerSettings struct {
	LoopInterval int                 `json:"loopInterval"`
	PauseOnHover bool                `json:"pauseOnHover"`
	RandomMode   bool                `json:"randomMode"`
	Items        []welcomeBannerItem `json:"items"`
}

type welcomeSettings struct {
	BadgeText         string                `json:"badgeText"`
	HeaderText        string                `json:"headerText"`
	TitleText         string                `json:"titleText"`
	SubtitleText      string                `json:"subtitleText"`
	CountdownTime     int                   `json:"countdownTime"`
	PrimaryColor      string                `json:"primaryColor"`
	SecondaryColor    string                `json:"secondaryColor"`
	GlowColor         string                `json:"glowColor"`
	BackgroundColor   string                `json:"backgroundColor"`
	FontFamily        string                `json:"fontFamily"`
	ParticlesCount    int                   `json:"particlesCount"`
	ParticlesSpeed    float64               `json:"particlesSpeed"`
	ParticlesGlow     bool                  `json:"particlesGlow"`
	BackgroundType    string                `json:"backgroundType"`
	BackgroundURL     string                `json:"backgroundUrl"`
	LogoURL           string                `json:"logoUrl"`
	LogoVisible       bool                  `json:"logoVisible"`
	MusicURL          string                `json:"musicUrl"`
	MusicEnabled      bool                  `json:"musicEnabled"`
	MusicVolume       float64               `json:"musicVolume"`
	Socials           []welcomeSocial       `json:"socials"`
	OBSScene          string                `json:"obsScene"`
	OBSAutoTransition bool                  `json:"obsAutoTransition"`
	Banner            welcomeBannerSettings `json:"banner"`
}

type persistedWelcomeTimer struct {
	RemainingSeconds    int                      `json:"remainingSeconds"`
	TotalDuration       int                      `json:"totalDuration"`
	Running             bool                     `json:"running"`
	EndsAt              string                   `json:"endsAt,omitempty"`
	Completed           bool                     `json:"completed"`
	TransitionAttempted bool                     `json:"transitionAttempted"`
	LastTransition      *welcomeTransitionResult `json:"lastTransition,omitempty"`
}

type welcomeTimerState struct {
	RemainingSeconds int    `json:"remainingSeconds"`
	TotalDuration    int    `json:"totalDuration"`
	Running          bool   `json:"running"`
	EndsAt           string `json:"endsAt,omitempty"`
	Completed        bool   `json:"completed"`
}

type welcomeTransitionResult struct {
	Success   bool   `json:"success"`
	SceneName string `json:"sceneName,omitempty"`
	Error     string `json:"error,omitempty"`
	At        string `json:"at,omitempty"`
}

type welcomeService struct {
	store       *eventStore
	switchScene func(context.Context, string) error
	mu          sync.Mutex
	settings    welcomeSettings
	timer       persistedWelcomeTimer
	endsAt      time.Time
}

func newWelcomeService(store *eventStore, switchScene func(context.Context, string) error) (*welcomeService, error) {
	service := &welcomeService{store: store, switchScene: switchScene}
	if err := service.load(); err != nil {
		return nil, err
	}
	return service, nil
}

func defaultWelcomeSettings() welcomeSettings {
	return welcomeSettings{
		BadgeText:       "🔴 EN VIVO PRONTO",
		HeaderText:      "STREAM COMENZANDO",
		TitleText:       "Empezamos Pronto",
		SubtitleText:    "Prepara tu café, ya casi empezamos...",
		CountdownTime:   300,
		PrimaryColor:    "#00f0ff",
		SecondaryColor:  "#bd00ff",
		GlowColor:       "#00f0ff",
		BackgroundColor: "#080710",
		FontFamily:      "Orbitron",
		ParticlesCount:  120,
		ParticlesSpeed:  1.5,
		ParticlesGlow:   true,
		BackgroundType:  "particles",
		LogoVisible:     true,
		MusicVolume:     0.5,
		Socials:         []welcomeSocial{},
		Banner: welcomeBannerSettings{
			LoopInterval: 4200,
			Items:        []welcomeBannerItem{},
		},
	}
}

func (s *welcomeService) load() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var settingsJSON string
	err := s.store.db.QueryRowContext(ctx, `SELECT settings_json FROM module_settings WHERE module_id = ?`, welcomeModuleID).Scan(&settingsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		s.settings = defaultWelcomeSettings()
		settingsBytes, marshalErr := json.Marshal(s.settings)
		if marshalErr != nil {
			return marshalErr
		}
		if _, err = s.store.db.ExecContext(ctx, `INSERT INTO module_settings(module_id, settings_json, updated_at) VALUES(?, ?, ?)`, welcomeModuleID, string(settingsBytes), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("guardar la configuración inicial de Bienvenidos y Banner: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("leer la configuración de Bienvenidos y Banner: %w", err)
	} else if err := json.Unmarshal([]byte(settingsJSON), &s.settings); err != nil {
		return fmt.Errorf("la configuración guardada de Bienvenidos y Banner no es válida: %w", err)
	} else {
		s.settings = normalizeWelcomeSettings(s.settings)
	}

	var runtimeJSON string
	err = s.store.db.QueryRowContext(ctx, `SELECT runtime_json FROM module_runtime WHERE module_id = ?`, welcomeModuleID).Scan(&runtimeJSON)
	if errors.Is(err, sql.ErrNoRows) {
		s.timer = persistedWelcomeTimer{RemainingSeconds: s.settings.CountdownTime, TotalDuration: s.settings.CountdownTime}
		return s.persistRuntime(ctx)
	}
	if err != nil {
		return fmt.Errorf("leer el estado del contador: %w", err)
	}
	if err := json.Unmarshal([]byte(runtimeJSON), &s.timer); err != nil {
		return fmt.Errorf("el estado guardado del contador no es válido: %w", err)
	}
	s.timer = normalizeWelcomeTimer(s.timer, s.settings.CountdownTime)
	if s.timer.Running {
		parsed, parseErr := time.Parse(time.RFC3339Nano, s.timer.EndsAt)
		if parseErr != nil {
			return fmt.Errorf("la fecha del contador guardado no es válida: %w", parseErr)
		}
		s.endsAt = parsed
	}
	return nil
}

func (s *welcomeService) reload() error {
	loaded, err := newWelcomeService(s.store, s.switchScene)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.settings = loaded.settings
	s.timer = loaded.timer
	s.endsAt = loaded.endsAt
	s.mu.Unlock()
	return nil
}

func (s *welcomeService) run(stop <-chan struct{}) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.advanceTimer()
		}
	}
}

func (s *welcomeService) snapshot() (welcomeSettings, welcomeTimerState, *welcomeTransitionResult) {
	s.advanceTimer()
	s.mu.Lock()
	defer s.mu.Unlock()
	var result *welcomeTransitionResult
	if s.timer.LastTransition != nil {
		copy := *s.timer.LastTransition
		result = &copy
	}
	return s.settings, s.timerSnapshotLocked(time.Now()), result
}

func (s *welcomeService) timerSnapshotLocked(now time.Time) welcomeTimerState {
	remaining := s.timer.RemainingSeconds
	endsAt := ""
	if s.timer.Running {
		remaining = int(math.Ceil(s.endsAt.Sub(now).Seconds()))
		if remaining < 0 {
			remaining = 0
		}
		endsAt = s.endsAt.UTC().Format(time.RFC3339Nano)
	}
	return welcomeTimerState{
		RemainingSeconds: remaining,
		TotalDuration:    s.timer.TotalDuration,
		Running:          s.timer.Running,
		EndsAt:           endsAt,
		Completed:        s.timer.Completed,
	}
}

func (s *welcomeService) advanceTimer() {
	s.mu.Lock()
	if !s.timer.Running || time.Now().Before(s.endsAt) {
		s.mu.Unlock()
		return
	}
	s.timer.Running = false
	s.timer.RemainingSeconds = 0
	s.timer.EndsAt = ""
	s.timer.Completed = true
	s.endsAt = time.Time{}
	shouldTransition := s.settings.OBSAutoTransition && strings.TrimSpace(s.settings.OBSScene) != "" && !s.timer.TransitionAttempted
	scene := strings.TrimSpace(s.settings.OBSScene)
	if shouldTransition {
		s.timer.TransitionAttempted = true
	}
	_ = s.persistRuntimeLocked(context.Background())
	s.mu.Unlock()
	if shouldTransition {
		go s.performTransition(scene)
	}
}

func (s *welcomeService) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.mu.Lock()
		settings := s.settings
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxWelcomeSettings)
	decoder := json.NewDecoder(r.Body)
	var patch map[string]json.RawMessage
	if err := decoder.Decode(&patch); err != nil || patch == nil {
		http.Error(w, "La configuración enviada no tiene un formato válido", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "La configuración contiene datos adicionales", http.StatusBadRequest)
		return
	}
	settings, err := s.updateSettings(r.Context(), patch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

func (s *welcomeService) updateSettings(ctx context.Context, patch map[string]json.RawMessage) (welcomeSettings, error) {
	known := map[string]bool{
		"badgeText": true, "headerText": true, "titleText": true, "subtitleText": true,
		"countdownTime": true, "primaryColor": true, "secondaryColor": true, "glowColor": true,
		"backgroundColor": true, "fontFamily": true, "particlesCount": true, "particlesSpeed": true,
		"particlesGlow": true, "backgroundType": true, "backgroundUrl": true, "logoUrl": true,
		"logoVisible": true, "musicUrl": true, "musicEnabled": true, "musicVolume": true,
		"socials": true, "obsScene": true, "obsAutoTransition": true, "banner": true,
	}
	for key := range patch {
		if !known[key] {
			return welcomeSettings{}, fmt.Errorf("el campo %q no está disponible en este módulo", key)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.settings
	currentBytes, err := json.Marshal(previous)
	if err != nil {
		return welcomeSettings{}, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(currentBytes, &merged); err != nil {
		return welcomeSettings{}, err
	}
	for key, value := range patch {
		merged[key] = value
	}
	mergedBytes, err := json.Marshal(merged)
	if err != nil {
		return welcomeSettings{}, err
	}
	var next welcomeSettings
	if err := json.Unmarshal(mergedBytes, &next); err != nil {
		return welcomeSettings{}, errors.New("uno de los valores de la configuración no es válido")
	}
	next = normalizeWelcomeSettings(next)
	if err := validateWelcomeSettings(next); err != nil {
		return welcomeSettings{}, err
	}
	nextTimer := s.timer
	if next.CountdownTime != previous.CountdownTime && !s.timer.Running {
		nextTimer.RemainingSeconds = next.CountdownTime
		nextTimer.TotalDuration = next.CountdownTime
		nextTimer.Completed = false
		nextTimer.TransitionAttempted = false
		nextTimer.EndsAt = ""
	}
	settingsBytes, err := json.Marshal(next)
	if err != nil {
		return welcomeSettings{}, err
	}
	runtimeBytes, err := json.Marshal(nextTimer)
	if err != nil {
		return welcomeSettings{}, err
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return welcomeSettings{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO module_settings(module_id, settings_json, updated_at) VALUES(?, ?, ?) ON CONFLICT(module_id) DO UPDATE SET settings_json = excluded.settings_json, updated_at = excluded.updated_at`, welcomeModuleID, string(settingsBytes), now); err != nil {
		return welcomeSettings{}, fmt.Errorf("no se pudo guardar la configuración: %w", err)
	}
	if nextTimer.RemainingSeconds != s.timer.RemainingSeconds || nextTimer.TotalDuration != s.timer.TotalDuration || nextTimer.Completed != s.timer.Completed {
		if _, err := tx.ExecContext(ctx, `INSERT INTO module_runtime(module_id, runtime_json, updated_at) VALUES(?, ?, ?) ON CONFLICT(module_id) DO UPDATE SET runtime_json = excluded.runtime_json, updated_at = excluded.updated_at`, welcomeModuleID, string(runtimeBytes), now); err != nil {
			return welcomeSettings{}, fmt.Errorf("no se pudo actualizar el contador: %w", err)
		}
	}
	for _, oldURL := range []string{previous.LogoURL, previous.BackgroundURL, previous.MusicURL} {
		if oldURL == "" || oldURL == next.LogoURL || oldURL == next.BackgroundURL || oldURL == next.MusicURL {
			continue
		}
		if id := welcomeAssetID(oldURL); id != "" {
			if _, err := tx.ExecContext(ctx, `DELETE FROM module_assets WHERE asset_id = ?`, id); err != nil {
				return welcomeSettings{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return welcomeSettings{}, err
	}
	s.settings = next
	if next.CountdownTime != previous.CountdownTime && !s.timer.Running {
		s.timer = nextTimer
		s.endsAt = time.Time{}
	}
	return s.settings, nil
}

func (s *welcomeService) handleTimer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		Action string `json:"action"`
		Value  int    `json:"value"`
	}
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "La acción del contador no es válida", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "La acción del contador contiene datos adicionales", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	now := time.Now()
	switch input.Action {
	case "start":
		remaining := s.timer.RemainingSeconds
		if s.timer.Running {
			remaining = int(math.Ceil(s.endsAt.Sub(now).Seconds()))
		}
		if input.Value > 0 && input.Value <= welcomeTimerMaxSecond {
			remaining = input.Value
		}
		if remaining <= 0 {
			remaining = s.settings.CountdownTime
			s.timer.TotalDuration = remaining
		}
		s.timer.RemainingSeconds = remaining
		s.timer.Running = true
		s.timer.Completed = false
		s.timer.TransitionAttempted = false
		s.endsAt = now.Add(time.Duration(remaining) * time.Second)
		s.timer.EndsAt = s.endsAt.UTC().Format(time.RFC3339Nano)
	case "pause":
		if s.timer.Running {
			s.timer.RemainingSeconds = maxInt(0, int(math.Ceil(s.endsAt.Sub(now).Seconds())))
		}
		s.timer.Running = false
		s.timer.EndsAt = ""
		s.endsAt = time.Time{}
	case "reset":
		s.timer = persistedWelcomeTimer{RemainingSeconds: s.settings.CountdownTime, TotalDuration: s.settings.CountdownTime}
		s.endsAt = time.Time{}
	case "sync":
		if input.Value < 0 || input.Value > welcomeTimerMaxSecond {
			s.mu.Unlock()
			http.Error(w, "El ajuste debe estar entre 0 y 24 horas", http.StatusBadRequest)
			return
		}
		s.timer.RemainingSeconds = input.Value
		if s.timer.TotalDuration < input.Value {
			s.timer.TotalDuration = input.Value
		}
		s.timer.Completed = false
		s.timer.TransitionAttempted = false
		if s.timer.Running {
			s.endsAt = now.Add(time.Duration(input.Value) * time.Second)
			s.timer.EndsAt = s.endsAt.UTC().Format(time.RFC3339Nano)
		}
	default:
		s.mu.Unlock()
		http.Error(w, "La acción del contador debe ser start, pause, reset o sync", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	err := s.persistRuntimeLocked(ctx)
	timer := s.timerSnapshotLocked(now)
	s.mu.Unlock()
	cancel()
	if err != nil {
		http.Error(w, "No se pudo guardar el estado del contador", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timer": timer})
}

func (s *welcomeService) handleUpload(w http.ResponseWriter, r *http.Request) {
	kind := strings.Trim(strings.ToLower(r.PathValue("kind")), " /")
	limits := map[string]int64{"logo": 8 << 20, "background-image": maxWelcomeImageBytes, "background-video": maxWelcomeVideoBytes, "bg-music": maxWelcomeAudioBytes}
	limit, ok := limits[kind]
	if !ok {
		http.Error(w, "El tipo de archivo no está permitido", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, "No se pudo leer el archivo o supera el tamaño permitido", http.StatusRequestEntityTooLarge)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	fieldName := "file"
	if kind == "background-video" {
		fieldName = "bgVideo"
	} else if kind == "background-image" {
		fieldName = "bgImage"
	} else if kind == "bg-music" {
		fieldName = "bgMusic"
	}
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		http.Error(w, "No se recibió el archivo esperado", http.StatusBadRequest)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		http.Error(w, "No se pudo leer el archivo", http.StatusBadRequest)
		return
	}
	if int64(len(content)) == 0 || int64(len(content)) > limit {
		http.Error(w, "El archivo está vacío o supera el tamaño permitido", http.StatusRequestEntityTooLarge)
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentKind := kind
	if kind == "bg-music" {
		contentKind = "music"
	}
	contentType, err := welcomeAssetType(contentKind, ext, content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	assetID, err := newWelcomeAssetID()
	if err != nil {
		http.Error(w, "No se pudo crear el identificador del archivo", http.StatusInternalServerError)
		return
	}
	assetURL := "/api/modules/welcome/assets/" + assetID
	field := map[string]string{"logo": "logoUrl", "background-image": "backgroundUrl", "background-video": "backgroundUrl", "bg-music": "musicUrl"}[kind]
	s.mu.Lock()
	previous := s.settings
	next := previous
	switch field {
	case "logoUrl":
		next.LogoURL = assetURL
	case "backgroundUrl":
		next.BackgroundURL = assetURL
		if kind == "background-video" {
			next.BackgroundType = "video"
		} else {
			next.BackgroundType = "image"
		}
	case "musicUrl":
		next.MusicURL = assetURL
	}
	next = normalizeWelcomeSettings(next)
	settingsBytes, _ := json.Marshal(next)
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if err == nil {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err = tx.ExecContext(r.Context(), `INSERT INTO module_assets(asset_id, content_type, original_name, byte_size, content, uploaded_at) VALUES(?, ?, ?, ?, ?, ?)`, assetID, contentType, kind+ext, len(content), content, now)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO module_settings(module_id, settings_json, updated_at) VALUES(?, ?, ?) ON CONFLICT(module_id) DO UPDATE SET settings_json = excluded.settings_json, updated_at = excluded.updated_at`, welcomeModuleID, string(settingsBytes), now)
		}
		if err == nil {
			for _, oldURL := range []string{previous.LogoURL, previous.BackgroundURL, previous.MusicURL} {
				if oldURL != "" && oldURL != next.LogoURL && oldURL != next.BackgroundURL && oldURL != next.MusicURL {
					if oldID := welcomeAssetID(oldURL); oldID != "" {
						_, err = tx.ExecContext(r.Context(), `DELETE FROM module_assets WHERE asset_id = ?`, oldID)
						if err != nil {
							break
						}
					}
				}
			}
		}
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
	}
	if err == nil {
		s.settings = next
	}
	s.mu.Unlock()
	if err != nil {
		http.Error(w, "No se pudo guardar el archivo en SQLite", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "url": assetURL, "settings": next})
}

func (s *welcomeService) handleAsset(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	if len(assetID) != 32 {
		http.NotFound(w, r)
		return
	}
	if _, err := hex.DecodeString(assetID); err != nil {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	var contentType string
	var content []byte
	var uploadedAt string
	err := s.store.db.QueryRowContext(ctx, `SELECT content_type, content, uploaded_at FROM module_assets WHERE asset_id = ?`, assetID).Scan(&contentType, &content, &uploadedAt)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "No se pudo leer el archivo del módulo", http.StatusServiceUnavailable)
		return
	}
	modifiedAt, _ := time.Parse(time.RFC3339Nano, uploadedAt)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, "media"+extensionForContentType(contentType), modifiedAt, bytes.NewReader(content))
}

func (s *welcomeService) performTransition(scene string) welcomeTransitionResult {
	result := welcomeTransitionResult{SceneName: scene, At: time.Now().UTC().Format(time.RFC3339Nano)}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	err := s.switchScene(ctx, scene)
	cancel()
	if err != nil {
		result.Error = "OBS no pudo cambiar a la escena configurada. Comprueba la conexión local y el nombre exacto de la escena."
	} else {
		result.Success = true
	}
	s.mu.Lock()
	s.timer.LastTransition = &result
	_ = s.persistRuntimeLocked(context.Background())
	s.mu.Unlock()
	return result
}

func (s *welcomeService) handleTransition(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	scene := strings.TrimSpace(s.settings.OBSScene)
	s.mu.Unlock()
	if scene == "" {
		http.Error(w, "Primero guarda el nombre exacto de la escena de OBS", http.StatusBadRequest)
		return
	}
	result := s.performTransition(scene)
	status := http.StatusOK
	if !result.Success {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, result)
}

func (s *welcomeService) persistRuntime(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persistRuntimeLocked(ctx)
}

func (s *welcomeService) persistRuntimeLocked(ctx context.Context) error {
	if s.timer.Running && !s.endsAt.IsZero() {
		s.timer.EndsAt = s.endsAt.UTC().Format(time.RFC3339Nano)
	}
	content, err := json.Marshal(s.timer)
	if err != nil {
		return err
	}
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO module_runtime(module_id, runtime_json, updated_at) VALUES(?, ?, ?) ON CONFLICT(module_id) DO UPDATE SET runtime_json = excluded.runtime_json, updated_at = excluded.updated_at`, welcomeModuleID, string(content), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func normalizeWelcomeSettings(s welcomeSettings) welcomeSettings {
	defaults := defaultWelcomeSettings()
	s.BadgeText = limitWelcomeText(s.BadgeText, 100, defaults.BadgeText)
	s.HeaderText = limitWelcomeText(s.HeaderText, 100, defaults.HeaderText)
	s.TitleText = limitWelcomeText(s.TitleText, 120, defaults.TitleText)
	s.SubtitleText = limitWelcomeText(s.SubtitleText, 300, defaults.SubtitleText)
	s.CountdownTime = clampInt(s.CountdownTime, 1, welcomeTimerMaxSecond, defaults.CountdownTime)
	s.PrimaryColor = normalizeColor(s.PrimaryColor, defaults.PrimaryColor)
	s.SecondaryColor = normalizeColor(s.SecondaryColor, defaults.SecondaryColor)
	s.GlowColor = normalizeColor(s.GlowColor, defaults.GlowColor)
	s.BackgroundColor = normalizeColor(s.BackgroundColor, defaults.BackgroundColor)
	fonts := map[string]bool{"Orbitron": true, "Rajdhani": true, "Share Tech Mono": true, "Inter": true, "Outfit": true, "Montserrat": true}
	if !fonts[s.FontFamily] {
		s.FontFamily = defaults.FontFamily
	}
	s.ParticlesCount = clampInt(s.ParticlesCount, 20, 250, defaults.ParticlesCount)
	if math.IsNaN(s.ParticlesSpeed) || math.IsInf(s.ParticlesSpeed, 0) {
		s.ParticlesSpeed = defaults.ParticlesSpeed
	}
	if s.ParticlesSpeed < 0.1 {
		s.ParticlesSpeed = 0.1
	}
	if s.ParticlesSpeed > 5 {
		s.ParticlesSpeed = 5
	}
	backgroundTypes := map[string]bool{"particles": true, "video": true, "image": true, "color": true}
	if !backgroundTypes[s.BackgroundType] {
		s.BackgroundType = defaults.BackgroundType
	}
	if !isWelcomeAssetURL(s.BackgroundURL) {
		s.BackgroundURL = ""
	}
	if !isWelcomeAssetURL(s.LogoURL) {
		s.LogoURL = ""
	}
	if !isWelcomeAssetURL(s.MusicURL) {
		s.MusicURL = ""
	}
	if math.IsNaN(s.MusicVolume) || math.IsInf(s.MusicVolume, 0) {
		s.MusicVolume = defaults.MusicVolume
	}
	s.MusicVolume = math.Max(0, math.Min(1, s.MusicVolume))
	s.OBSScene = limitWelcomeText(strings.TrimSpace(s.OBSScene), 128, "")
	if len(s.Socials) > 16 {
		s.Socials = s.Socials[:16]
	}
	for index := range s.Socials {
		s.Socials[index].Platform = normalizeWelcomePlatform(s.Socials[index].Platform, true)
		s.Socials[index].ID = limitWelcomeText(s.Socials[index].ID, 64, fmt.Sprintf("social-%d", index+1))
		s.Socials[index].Handle = limitWelcomeText(strings.TrimSpace(s.Socials[index].Handle), 256, "")
	}
	s.Banner.LoopInterval = clampInt(s.Banner.LoopInterval, 1500, 60000, defaults.Banner.LoopInterval)
	if len(s.Banner.Items) > 16 {
		s.Banner.Items = s.Banner.Items[:16]
	}
	for index := range s.Banner.Items {
		s.Banner.Items[index].Platform = normalizeWelcomePlatform(s.Banner.Items[index].Platform, false)
		s.Banner.Items[index].ID = limitWelcomeText(s.Banner.Items[index].ID, 64, fmt.Sprintf("banner-%d", index+1))
		if strings.TrimSpace(s.Banner.Items[index].Label) == "" {
			s.Banner.Items[index].Label = welcomePlatformLabel(s.Banner.Items[index].Platform)
		}
		s.Banner.Items[index].Label = limitWelcomeText(strings.TrimSpace(s.Banner.Items[index].Label), 64, "Redes")
		s.Banner.Items[index].Handle = limitWelcomeText(strings.TrimSpace(s.Banner.Items[index].Handle), 256, "")
	}
	if s.Socials == nil {
		s.Socials = []welcomeSocial{}
	}
	if s.Banner.Items == nil {
		s.Banner.Items = []welcomeBannerItem{}
	}
	return s
}

func validateWelcomeSettings(s welcomeSettings) error {
	for name, color := range map[string]string{"primaryColor": s.PrimaryColor, "secondaryColor": s.SecondaryColor, "glowColor": s.GlowColor, "backgroundColor": s.BackgroundColor} {
		if !welcomeColorPattern.MatchString(color) {
			return fmt.Errorf("el color %s debe tener formato hexadecimal #RRGGBB", name)
		}
	}
	return nil
}

func normalizeWelcomeTimer(timer persistedWelcomeTimer, defaultDuration int) persistedWelcomeTimer {
	if timer.TotalDuration < 1 || timer.TotalDuration > welcomeTimerMaxSecond {
		timer.TotalDuration = defaultDuration
	}
	if timer.RemainingSeconds < 0 {
		timer.RemainingSeconds = 0
	}
	if timer.RemainingSeconds > welcomeTimerMaxSecond {
		timer.RemainingSeconds = welcomeTimerMaxSecond
	}
	if timer.Running && timer.EndsAt == "" {
		timer.Running = false
		timer.RemainingSeconds = defaultDuration
	}
	return timer
}

func welcomeAssetType(kind, ext string, content []byte) (string, error) {
	ext = strings.ToLower(ext)
	allowed := map[string]map[string]string{
		"logo":             {".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".gif": "image/gif"},
		"background-image": {".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp"},
		"background-video": {".mp4": "video/mp4", ".webm": "video/webm"},
		"music":            {".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg"},
	}
	contentType, ok := allowed[kind][ext]
	if !ok {
		return "", errors.New("usa un formato compatible: PNG, JPEG, WebP, MP4, WebM, MP3, WAV u OGG")
	}
	valid := false
	switch contentType {
	case "image/png":
		valid = len(content) >= 8 && bytes.Equal(content[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10})
	case "image/jpeg":
		valid = len(content) >= 3 && bytes.Equal(content[:3], []byte{0xff, 0xd8, 0xff})
	case "image/webp":
		valid = len(content) >= 12 && string(content[:4]) == "RIFF" && string(content[8:12]) == "WEBP"
	case "image/gif":
		valid = len(content) >= 6 && (string(content[:6]) == "GIF87a" || string(content[:6]) == "GIF89a")
	case "video/mp4":
		valid = len(content) >= 12 && string(content[4:8]) == "ftyp"
	case "video/webm":
		valid = len(content) >= 4 && bytes.Equal(content[:4], []byte{0x1a, 0x45, 0xdf, 0xa3})
	case "audio/mpeg":
		valid = len(content) >= 3 && (string(content[:3]) == "ID3" || content[0] == 0xff && content[1]&0xe0 == 0xe0)
	case "audio/wav":
		valid = len(content) >= 12 && string(content[:4]) == "RIFF" && string(content[8:12]) == "WAVE"
	case "audio/ogg":
		valid = len(content) >= 4 && string(content[:4]) == "OggS"
	}
	if !valid {
		return "", errors.New("el contenido del archivo no coincide con su extensión")
	}
	return contentType, nil
}

func welcomeAssetID(assetURL string) string {
	match := welcomeAssetPattern.FindStringSubmatch(assetURL)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func isWelcomeAssetURL(assetURL string) bool { return welcomeAssetID(assetURL) != "" }

func newWelcomeAssetID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func extensionForContentType(contentType string) string {
	if ext, _ := mime.ExtensionsByType(contentType); len(ext) > 0 {
		return ext[0]
	}
	return ".bin"
}

func normalizeColor(value, fallback string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if welcomeColorPattern.MatchString(value) {
		return value
	}
	return fallback
}

func limitWelcomeText(value string, maxRunes int, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	runes := []rune(value)
	if len(runes) > maxRunes {
		value = string(runes[:maxRunes])
	}
	return value
}

func normalizeWelcomePlatform(value string, overlay bool) string {
	value = strings.ToLower(strings.TrimSpace(value))
	allowed := map[string]bool{"instagram": true, "facebook": true, "tiktok": true, "telegram": true, "web": true}
	if overlay {
		allowed = map[string]bool{"twitch": true, "youtube": true, "twitter": true, "x": true, "instagram": true, "tiktok": true, "facebook": true, "custom": true}
	}
	if allowed[value] {
		return value
	}
	if overlay {
		return "custom"
	}
	return "web"
}

func welcomePlatformLabel(platform string) string {
	labels := map[string]string{"instagram": "Instagram", "facebook": "Facebook", "tiktok": "TikTok", "telegram": "Telegram", "web": "Web"}
	if label := labels[platform]; label != "" {
		return label
	}
	return "Redes"
}

func clampInt(value, minimum, maximum, fallback int) int {
	if value < minimum || value > maximum {
		return fallback
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
