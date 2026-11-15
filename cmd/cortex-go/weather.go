package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"
)

const (
	weatherModuleID       = "weather-time"
	weatherCacheLifetime  = 10 * time.Minute
	weatherRefreshEvery   = 10 * time.Minute
	weatherMaxLocations   = 20
	weatherMaxResponse    = 1 << 20
	weatherMaxConfigBytes = 64 << 10
)

var (
	weatherColorPattern      = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	weatherLocationIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,48}$`)
	weatherClientIDPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)
	weatherNormalizedPattern = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	weatherAccentRemover     = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n", "Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ü", "u", "Ñ", "n")
)

type weatherLocation struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	Timezone    string `json:"timezone"`
	City        string `json:"city"`
	Visible     bool   `json:"visible"`
	Flag        string `json:"flag"`
	ShowClock   bool   `json:"showClock"`
	ShowWeather bool   `json:"showWeather"`
}

type weatherConfig struct {
	Locations      []weatherLocation `json:"locations"`
	TimeFormat     string            `json:"timeFormat"`
	CycleDuration  int               `json:"cycleDuration"`
	ThemeColor     string            `json:"themeColor"`
	ThemePreset    string            `json:"themePreset"`
	FontFamily     string            `json:"fontFamily"`
	TransitionType string            `json:"transitionType"`
	CyclePaused    bool              `json:"cyclePaused"`
	ShowLiveBar    bool              `json:"showLiveBar"`
	LiveBarText    string            `json:"liveBarText"`
	LayoutStyle    string            `json:"layoutStyle"`
	Position       string            `json:"position"`
}

type weatherConfigView struct {
	weatherConfig
	ActiveViewIndex int `json:"activeViewIndex"`
}

type weatherCondition struct {
	City        string `json:"city"`
	Temp        string `json:"temp"`
	WeatherCode *int   `json:"weatherCode"`
	IsDay       *int   `json:"isDay"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	Status      string `json:"status"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	Error       string `json:"error,omitempty"`
}

type weatherRuntime struct {
	Conditions      map[string]weatherCondition `json:"conditions"`
	ActiveViewIndex int                         `json:"activeViewIndex"`
	LastAttempt     string                      `json:"lastAttempt,omitempty"`
	LastRefresh     string                      `json:"lastRefresh,omitempty"`
	LastError       string                      `json:"lastError,omitempty"`
}

type weatherClient struct {
	id       string
	typeName string
	events   chan weatherEvent
}

type weatherEvent struct {
	name    string
	payload json.RawMessage
}

type weatherConnectionStatus struct {
	Overlays   int `json:"overlays"`
	Dashboards int `json:"dashboards"`
}

type weatherService struct {
	store  *eventStore
	client *http.Client

	mu           sync.Mutex
	config       weatherConfig
	runtime      weatherRuntime
	clients      map[string]*weatherClient
	refreshing   bool
	lastCycleAt  time.Time
	ctx          context.Context
	geoCache     map[string]weatherCoordinates
	weatherCache map[string]weatherCacheEntry
}

type weatherCoordinates struct {
	Latitude  float64
	Longitude float64
}

type weatherCacheEntry struct {
	condition weatherCondition
	fetchedAt time.Time
}

func defaultWeatherConfig() weatherConfig {
	return weatherConfig{
		Locations: []weatherLocation{
			{ID: "loc2", Text: "Gold Coast", Timezone: "Australia/Brisbane", City: "Gold Coast", Visible: true, Flag: "au", ShowClock: true, ShowWeather: true},
			{ID: "loc1", Text: "C. del Uruguay", Timezone: "America/Argentina/Buenos_Aires", City: "Concepcion del Uruguay", Visible: true, Flag: "ar", ShowClock: true, ShowWeather: true},
			{ID: "loc3", Text: "San Justo", Timezone: "America/Argentina/Buenos_Aires", City: "San Justo, Entre Rios", Visible: true, Flag: "ar", ShowClock: true, ShowWeather: true},
		},
		TimeFormat: "12h", CycleDuration: 10, ThemeColor: "#f59e0b", ThemePreset: "classic",
		FontFamily: "Roboto", TransitionType: "flip", CyclePaused: false,
		ShowLiveBar: true, LiveBarText: "EN VIVO", LayoutStyle: "card", Position: "top-left",
	}
}

func newWeatherService(store *eventStore) (*weatherService, error) {
	service := &weatherService{
		store:        store,
		client:       &http.Client{Timeout: 8 * time.Second},
		config:       defaultWeatherConfig(),
		clients:      make(map[string]*weatherClient),
		geoCache:     make(map[string]weatherCoordinates),
		weatherCache: make(map[string]weatherCacheEntry),
		runtime:      weatherRuntime{Conditions: make(map[string]weatherCondition), ActiveViewIndex: 1},
		lastCycleAt:  time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var settingsJSON string
	err := store.db.QueryRowContext(ctx, `SELECT settings_json FROM module_settings WHERE module_id = ?`, weatherModuleID).Scan(&settingsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		if err := service.persistConfig(ctx, service.config); err != nil {
			return nil, fmt.Errorf("guardar la configuración inicial de Clima y Tiempo: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("leer la configuración de Clima y Tiempo: %w", err)
	} else {
		if err := json.Unmarshal([]byte(settingsJSON), &service.config); err != nil {
			return nil, fmt.Errorf("decodificar la configuración de Clima y Tiempo: %w", err)
		}
	}
	if err := validateWeatherConfig(service.config); err != nil {
		return nil, fmt.Errorf("la configuración guardada de Clima y Tiempo no es válida: %w", err)
	}

	var runtimeJSON string
	err = store.db.QueryRowContext(ctx, `SELECT runtime_json FROM module_runtime WHERE module_id = ?`, weatherModuleID).Scan(&runtimeJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return service, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leer el estado de Clima y Tiempo: %w", err)
	}
	if err := json.Unmarshal([]byte(runtimeJSON), &service.runtime); err != nil {
		return nil, fmt.Errorf("decodificar el estado de Clima y Tiempo: %w", err)
	}
	if service.runtime.Conditions == nil {
		service.runtime.Conditions = make(map[string]weatherCondition)
	}
	if service.runtime.ActiveViewIndex < 0 {
		service.runtime.ActiveViewIndex = 0
	}
	return service, nil
}

func (s *weatherService) persistConfig(ctx context.Context, config weatherConfig) error {
	content, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO module_settings(module_id, settings_json, updated_at)
		VALUES(?, ?, ?) ON CONFLICT(module_id) DO UPDATE SET settings_json = excluded.settings_json, updated_at = excluded.updated_at`,
		weatherModuleID, string(content), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *weatherService) persistRuntime(ctx context.Context, runtime weatherRuntime) error {
	content, err := json.Marshal(runtime)
	if err != nil {
		return err
	}
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO module_runtime(module_id, runtime_json, updated_at)
		VALUES(?, ?, ?) ON CONFLICT(module_id) DO UPDATE SET runtime_json = excluded.runtime_json, updated_at = excluded.updated_at`,
		weatherModuleID, string(content), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *weatherService) snapshot() (weatherConfigView, weatherRuntime, weatherConnectionStatus, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configViewLocked(), cloneWeatherRuntime(s.runtime), s.connectionStatusLocked(), s.refreshing
}

func (s *weatherService) configViewLocked() weatherConfigView {
	return weatherConfigView{weatherConfig: s.config, ActiveViewIndex: s.runtime.ActiveViewIndex}
}

func cloneWeatherRuntime(runtime weatherRuntime) weatherRuntime {
	copyRuntime := runtime
	copyRuntime.Conditions = make(map[string]weatherCondition, len(runtime.Conditions))
	for id, condition := range runtime.Conditions {
		copyRuntime.Conditions[id] = condition
	}
	return copyRuntime
}

func (s *weatherService) handleState(w http.ResponseWriter, _ *http.Request) {
	config, runtime, connections, refreshing := s.snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true, "config": config, "weather": runtime.Conditions,
		"connections": connections, "refreshing": refreshing,
		"lastAttempt": runtime.LastAttempt, "lastRefresh": runtime.LastRefresh, "lastError": runtime.LastError,
	})
}

func (s *weatherService) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		config, _, _, _ := s.snapshot()
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "config": config})
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	patch, err := decodeWeatherConfigPatch(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	config, citiesChanged, err := s.updateConfig(r.Context(), patch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.broadcast("configUpdate", config)
	if citiesChanged {
		s.startRefresh(false)
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "config": config})
}

func decodeWeatherConfigPatch(w http.ResponseWriter, r *http.Request) (map[string]json.RawMessage, error) {
	r.Body = http.MaxBytesReader(w, r.Body, weatherMaxConfigBytes)
	decoder := json.NewDecoder(r.Body)
	var request map[string]json.RawMessage
	if err := decoder.Decode(&request); err != nil {
		return nil, errors.New("la configuración de Clima y Tiempo no es un JSON válido")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("la configuración contiene datos adicionales")
	}
	if len(request) == 0 {
		return nil, errors.New("la configuración está vacía")
	}
	if changes, ok := request["changes"]; ok {
		if len(request) != 1 {
			return nil, errors.New("envía los cambios dentro del campo changes")
		}
		if err := json.Unmarshal(changes, &request); err != nil || request == nil {
			return nil, errors.New("el campo changes no es válido")
		}
	}
	allowed := map[string]bool{
		"locations": true, "timeFormat": true, "cycleDuration": true, "themeColor": true,
		"themePreset": true, "fontFamily": true, "transitionType": true, "cyclePaused": true,
		"showLiveBar": true, "liveBarText": true, "layoutStyle": true, "position": true,
	}
	for key := range request {
		if !allowed[key] {
			return nil, fmt.Errorf("el campo %q no pertenece al módulo Clima y Tiempo", key)
		}
	}
	return request, nil
}

func (s *weatherService) updateConfig(ctx context.Context, patch map[string]json.RawMessage) (weatherConfigView, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	encoded, err := json.Marshal(s.config)
	if err != nil {
		return weatherConfigView{}, false, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &merged); err != nil {
		return weatherConfigView{}, false, err
	}
	for key, value := range patch {
		merged[key] = value
	}
	encoded, err = json.Marshal(merged)
	if err != nil {
		return weatherConfigView{}, false, err
	}
	var next weatherConfig
	if err := json.Unmarshal(encoded, &next); err != nil {
		return weatherConfigView{}, false, errors.New("uno o más valores de la configuración tienen un tipo incorrecto")
	}
	if err := validateWeatherConfig(next); err != nil {
		return weatherConfigView{}, false, err
	}
	citiesChanged := weatherCitySignature(s.config.Locations) != weatherCitySignature(next.Locations)
	previousLocations := make(map[string]weatherLocation, len(s.config.Locations))
	for _, location := range s.config.Locations {
		previousLocations[location.ID] = location
	}
	writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := s.persistConfig(writeCtx, next); err != nil {
		return weatherConfigView{}, false, fmt.Errorf("no se pudo guardar la configuración de Clima y Tiempo: %w", err)
	}
	s.config = next
	queueLen := weatherViewCount(next.Locations)
	if queueLen == 0 {
		s.runtime.ActiveViewIndex = 0
	} else if s.runtime.ActiveViewIndex >= queueLen {
		s.runtime.ActiveViewIndex = s.runtime.ActiveViewIndex % queueLen
	}
	if citiesChanged {
		valid := make(map[string]bool, len(next.Locations))
		for _, location := range next.Locations {
			valid[location.ID] = true
			previous, existed := previousLocations[location.ID]
			if !existed || !strings.EqualFold(strings.TrimSpace(previous.City), strings.TrimSpace(location.City)) {
				delete(s.runtime.Conditions, location.ID)
			}
		}
		for id := range s.runtime.Conditions {
			if !valid[id] {
				delete(s.runtime.Conditions, id)
			}
		}
	}
	s.lastCycleAt = time.Now()
	if err := s.persistRuntime(writeCtx, cloneWeatherRuntime(s.runtime)); err != nil {
		return weatherConfigView{}, false, fmt.Errorf("se guardó la configuración, pero no se pudo actualizar el estado: %w", err)
	}
	return s.configViewLocked(), citiesChanged, nil
}

func validateWeatherConfig(config weatherConfig) error {
	if len(config.Locations) > weatherMaxLocations {
		return fmt.Errorf("puedes configurar hasta %d ubicaciones", weatherMaxLocations)
	}
	seen := make(map[string]bool, len(config.Locations))
	for index, location := range config.Locations {
		if !weatherLocationIDPattern.MatchString(location.ID) || seen[location.ID] {
			return fmt.Errorf("el identificador de la ubicación %d no es válido o está repetido", index+1)
		}
		seen[location.ID] = true
		if strings.TrimSpace(location.Text) == "" || len([]rune(location.Text)) > 48 {
			return fmt.Errorf("la etiqueta de la ubicación %d debe tener entre 1 y 48 caracteres", index+1)
		}
		if _, err := time.LoadLocation(location.Timezone); err != nil {
			return fmt.Errorf("la zona horaria de la ubicación %d no es válida", index+1)
		}
		if len([]rune(location.City)) > 100 {
			return fmt.Errorf("la ciudad de la ubicación %d supera los 100 caracteres", index+1)
		}
		flag := strings.ToLower(location.Flag)
		if flag != "none" && (len(flag) != 2 || flag[0] < 'a' || flag[0] > 'z' || flag[1] < 'a' || flag[1] > 'z') {
			return fmt.Errorf("la bandera de la ubicación %d no es válida", index+1)
		}
	}
	if config.TimeFormat != "12h" && config.TimeFormat != "24h" {
		return errors.New("el formato de hora no es válido")
	}
	if config.CycleDuration < 3 || config.CycleDuration > 60 {
		return errors.New("la duración de cada vista debe estar entre 3 y 60 segundos")
	}
	if !weatherColorPattern.MatchString(config.ThemeColor) {
		return errors.New("el color de tema debe usar el formato hexadecimal #RRGGBB")
	}
	if !weatherContains(config.ThemePreset, "classic", "glass", "cyber", "dark", "retro") {
		return errors.New("el preset visual no es válido")
	}
	if !weatherContains(config.FontFamily, "Roboto", "Inter", "Montserrat", "Poppins", "Oswald", "Share Tech Mono") {
		return errors.New("la tipografía no es válida")
	}
	if !weatherContains(config.TransitionType, "slide-up", "slide-left", "slide-right", "fade", "zoom", "flip") {
		return errors.New("la transición no es válida")
	}
	if !weatherContains(config.LayoutStyle, "card", "ticker") {
		return errors.New("el diseño del overlay no es válido")
	}
	if !weatherContains(config.Position, "top-left", "top-right", "bottom-left", "bottom-right") {
		return errors.New("la posición del overlay no es válida")
	}
	if len([]rune(config.LiveBarText)) > 32 {
		return errors.New("el texto de la etiqueta EN VIVO supera los 32 caracteres")
	}
	return nil
}

func weatherContains(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func weatherCitySignature(locations []weatherLocation) string {
	var builder strings.Builder
	for _, location := range locations {
		builder.WriteString(location.ID)
		builder.WriteByte(':')
		builder.WriteString(strings.ToLower(strings.TrimSpace(location.City)))
		builder.WriteByte('|')
	}
	return builder.String()
}

func weatherViewCount(locations []weatherLocation) int {
	count := 0
	for _, location := range locations {
		if !location.Visible {
			continue
		}
		if location.ShowClock {
			count++
		}
		if location.ShowWeather && strings.TrimSpace(location.City) != "" {
			count++
		}
	}
	return count
}

func (s *weatherService) handleRefresh(w http.ResponseWriter, _ *http.Request) {
	started := s.startRefresh(true)
	status := http.StatusAccepted
	if !started {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"success": true, "refreshing": true, "alreadyRunning": !started})
}

func (s *weatherService) startRefresh(bypassCache bool) bool {
	s.mu.Lock()
	if s.refreshing {
		s.mu.Unlock()
		return false
	}
	s.refreshing = true
	s.runtime.LastAttempt = time.Now().UTC().Format(time.RFC3339)
	config := s.config
	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	runtime := cloneWeatherRuntime(s.runtime)
	s.mu.Unlock()
	s.broadcast("weatherUpdate", runtime)
	go s.refresh(ctx, config, bypassCache)
	return true
}

func (s *weatherService) refresh(ctx context.Context, config weatherConfig, bypassCache bool) {
	queries := make(map[string]string)
	for _, location := range config.Locations {
		city := strings.TrimSpace(location.City)
		if city != "" {
			queries[weatherCacheKey(city)] = city
		}
	}
	type result struct {
		key       string
		condition weatherCondition
		err       error
	}
	results := make(chan result, len(queries))
	semaphore := make(chan struct{}, 4)
	var group sync.WaitGroup
	for key, city := range queries {
		key, city := key, city
		group.Add(1)
		go func() {
			defer group.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				results <- result{key: key, err: ctx.Err()}
				return
			}
			defer func() { <-semaphore }()
			condition, err := s.fetchCityWeather(ctx, city, bypassCache)
			results <- result{key: key, condition: condition, err: err}
		}()
	}
	group.Wait()
	close(results)
	fetched := make(map[string]result, len(queries))
	for item := range results {
		fetched[item.key] = item
	}

	now := time.Now().UTC().Format(time.RFC3339)
	failed := 0
	var firstErr string
	s.mu.Lock()
	if weatherCitySignature(s.config.Locations) != weatherCitySignature(config.Locations) {
		s.refreshing = false
		runtime := cloneWeatherRuntime(s.runtime)
		s.mu.Unlock()
		s.broadcast("weatherUpdate", runtime)
		s.startRefresh(false)
		return
	}
	validLocations := make(map[string]weatherLocation, len(config.Locations))
	for _, location := range config.Locations {
		validLocations[location.ID] = location
	}
	for id := range s.runtime.Conditions {
		if _, exists := validLocations[id]; !exists {
			delete(s.runtime.Conditions, id)
		}
	}
	for _, location := range config.Locations {
		city := strings.TrimSpace(location.City)
		if city == "" {
			delete(s.runtime.Conditions, location.ID)
			continue
		}
		item := fetched[weatherCacheKey(city)]
		if item.err == nil {
			condition := item.condition
			condition.City = city
			condition.Status = "ok"
			condition.UpdatedAt = now
			condition.Error = ""
			s.runtime.Conditions[location.ID] = condition
			continue
		}
		failed++
		if firstErr == "" {
			firstErr = shortWeatherError(item.err)
		}
		previous, exists := s.runtime.Conditions[location.ID]
		if exists && strings.EqualFold(strings.TrimSpace(previous.City), city) && previous.Temp != "" && previous.Temp != "--°C" {
			previous.Status = "stale"
			previous.Error = shortWeatherError(item.err)
			s.runtime.Conditions[location.ID] = previous
		} else {
			s.runtime.Conditions[location.ID] = weatherCondition{City: city, Temp: "--°C", Status: "unavailable", Error: shortWeatherError(item.err)}
		}
	}
	s.refreshing = false
	s.runtime.LastAttempt = now
	if failed == 0 {
		s.runtime.LastRefresh = now
		s.runtime.LastError = ""
	} else if failed < len(queries) {
		s.runtime.LastRefresh = now
		s.runtime.LastError = fmt.Sprintf("No se pudo actualizar %d de %d ciudades: %s", failed, len(queries), firstErr)
	} else if len(queries) > 0 {
		s.runtime.LastError = fmt.Sprintf("No se pudo actualizar el clima: %s", firstErr)
	} else {
		s.runtime.LastRefresh = now
		s.runtime.LastError = ""
	}
	runtime := cloneWeatherRuntime(s.runtime)
	s.mu.Unlock()

	writeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	if err := s.persistRuntime(writeCtx, runtime); err != nil {
		// El panel conserva el estado en memoria; el error se deja en el registro del proceso.
		fmt.Printf("weather: no se pudo guardar el estado meteorológico: %v\n", err)
	}
	cancel()
	s.broadcast("weatherUpdate", runtime)
}

func shortWeatherError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 180 {
		message = message[:180]
	}
	return message
}

func weatherCacheKey(city string) string {
	normalized := weatherAccentRemover.Replace(strings.ToLower(strings.TrimSpace(city)))
	return strings.TrimSpace(weatherNormalizedPattern.ReplaceAllString(normalized, " "))
}

func (s *weatherService) fetchCityWeather(ctx context.Context, city string, bypassCache bool) (weatherCondition, error) {
	key := weatherCacheKey(city)
	now := time.Now()
	if !bypassCache {
		s.mu.Lock()
		cached, exists := s.weatherCache[key]
		s.mu.Unlock()
		if exists && now.Sub(cached.fetchedAt) < weatherCacheLifetime {
			return cached.condition, nil
		}
	}
	coordinates, err := s.geocodeCity(ctx, city)
	if err != nil {
		return weatherCondition{}, err
	}
	condition, err := s.fetchCurrentWeather(ctx, coordinates)
	if err != nil {
		return weatherCondition{}, err
	}
	condition.City = city
	condition.Status = "ok"
	condition.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	s.weatherCache[key] = weatherCacheEntry{condition: condition, fetchedAt: time.Now()}
	s.mu.Unlock()
	return condition, nil
}

func (s *weatherService) geocodeCity(ctx context.Context, city string) (weatherCoordinates, error) {
	key := weatherCacheKey(city)
	s.mu.Lock()
	coordinates, exists := s.geoCache[key]
	s.mu.Unlock()
	if exists {
		return coordinates, nil
	}
	searchName := strings.TrimSpace(city)
	qualifier := ""
	if comma := strings.IndexByte(searchName, ','); comma >= 0 {
		qualifier = strings.TrimSpace(searchName[comma+1:])
		searchName = strings.TrimSpace(searchName[:comma])
	}
	query := url.Values{}
	query.Set("name", searchName)
	query.Set("count", "20")
	query.Set("language", "es")
	query.Set("format", "json")
	var response struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Admin1    string  `json:"admin1"`
			Admin2    string  `json:"admin2"`
			Country   string  `json:"country"`
		} `json:"results"`
	}
	if err := s.getJSON(ctx, "https://geocoding-api.open-meteo.com/v1/search?"+query.Encode(), &response); err != nil {
		return weatherCoordinates{}, fmt.Errorf("geocodificación de %q: %w", city, err)
	}
	if len(response.Results) == 0 {
		return weatherCoordinates{}, fmt.Errorf("no se encontró la ciudad %q", city)
	}
	selected := response.Results[0]
	if qualifier != "" {
		wanted := weatherCacheKey(qualifier)
		for _, candidate := range response.Results {
			areas := []string{candidate.Admin1, candidate.Admin2, candidate.Country}
			for _, area := range areas {
				if weatherCacheKey(area) == wanted {
					selected = candidate
					goto matched
				}
			}
		}
	}
matched:
	coordinates = weatherCoordinates{Latitude: selected.Latitude, Longitude: selected.Longitude}
	s.mu.Lock()
	s.geoCache[key] = coordinates
	s.mu.Unlock()
	return coordinates, nil
}

func (s *weatherService) fetchCurrentWeather(ctx context.Context, coordinates weatherCoordinates) (weatherCondition, error) {
	query := url.Values{}
	query.Set("latitude", fmt.Sprintf("%.6f", coordinates.Latitude))
	query.Set("longitude", fmt.Sprintf("%.6f", coordinates.Longitude))
	query.Set("current", "temperature_2m,weather_code,is_day")
	query.Set("temperature_unit", "celsius")
	query.Set("timezone", "auto")
	var response struct {
		Current *struct {
			Temperature float64 `json:"temperature_2m"`
			WeatherCode int     `json:"weather_code"`
			IsDay       int     `json:"is_day"`
		} `json:"current"`
	}
	if err := s.getJSON(ctx, "https://api.open-meteo.com/v1/forecast?"+query.Encode(), &response); err != nil {
		return weatherCondition{}, fmt.Errorf("consulta meteorológica: %w", err)
	}
	if response.Current == nil || math.IsNaN(response.Current.Temperature) || math.IsInf(response.Current.Temperature, 0) {
		return weatherCondition{}, errors.New("la respuesta meteorológica no contiene condiciones actuales válidas")
	}
	code := response.Current.WeatherCode
	day := response.Current.IsDay
	description, icon := weatherDescription(code, day == 1)
	return weatherCondition{
		Temp:        fmt.Sprintf("%d°C", int(math.Round(response.Current.Temperature))),
		WeatherCode: &code, IsDay: &day, Icon: icon, Description: description, Status: "ok",
	}, nil
}

func (s *weatherService) getJSON(ctx context.Context, target string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Cortex-Go/1.0 (+weather overlay)")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("el proveedor respondió HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, weatherMaxResponse))
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("respuesta JSON no válida: %w", err)
	}
	return nil
}

func weatherDescription(code int, isDay bool) (string, string) {
	dayIcon, nightIcon, description := "🌡️", "🌙", "Condición actual"
	switch code {
	case 0:
		dayIcon, nightIcon, description = "☀️", "🌙", "Despejado"
	case 1:
		dayIcon, nightIcon, description = "🌤️", "🌙", "Mayormente despejado"
	case 2:
		dayIcon, nightIcon, description = "⛅", "☁️", "Parcialmente nublado"
	case 3:
		dayIcon, nightIcon, description = "☁️", "☁️", "Nublado"
	case 45, 48:
		dayIcon, nightIcon, description = "🌫️", "🌫️", "Niebla"
	case 51, 53, 55, 56, 57:
		dayIcon, nightIcon, description = "🌦️", "🌧️", "Llovizna"
	case 61, 63, 65, 66, 67:
		dayIcon, nightIcon, description = "🌧️", "🌧️", "Lluvia"
	case 71, 73, 75, 77:
		dayIcon, nightIcon, description = "❄️", "❄️", "Nieve"
	case 80, 81, 82:
		dayIcon, nightIcon, description = "🌦️", "🌧️", "Chubascos"
	case 85, 86:
		dayIcon, nightIcon, description = "🌨️", "🌨️", "Chubascos de nieve"
	case 95, 96, 99:
		dayIcon, nightIcon, description = "⛈️", "⛈️", "Tormenta"
	}
	if isDay {
		return description, dayIcon
	}
	return description, nightIcon
}

func (s *weatherService) handleControl(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request struct {
		Action string `json:"action"`
		Index  *int   `json:"index"`
	}
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "La acción de reproducción no es válida", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "La solicitud contiene datos adicionales", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	queueLen := weatherViewCount(s.config.Locations)
	configChanged := false
	switch request.Action {
	case "toggle":
		s.config.CyclePaused = !s.config.CyclePaused
		configChanged = true
	case "pause":
		if !s.config.CyclePaused {
			s.config.CyclePaused = true
			configChanged = true
		}
	case "resume":
		if s.config.CyclePaused {
			s.config.CyclePaused = false
			configChanged = true
		}
	case "next", "previous", "select":
		if queueLen == 0 {
			s.mu.Unlock()
			http.Error(w, "No hay vistas habilitadas para reproducir", http.StatusConflict)
			return
		}
		switch request.Action {
		case "next":
			s.runtime.ActiveViewIndex = (s.runtime.ActiveViewIndex + 1) % queueLen
		case "previous":
			s.runtime.ActiveViewIndex = (s.runtime.ActiveViewIndex + queueLen - 1) % queueLen
		case "select":
			if request.Index == nil || *request.Index < 0 || *request.Index >= queueLen {
				s.mu.Unlock()
				http.Error(w, "La vista seleccionada está fuera de rango", http.StatusBadRequest)
				return
			}
			s.runtime.ActiveViewIndex = *request.Index
		}
		if !s.config.CyclePaused {
			s.config.CyclePaused = true
			configChanged = true
		}
	default:
		s.mu.Unlock()
		http.Error(w, "La acción debe ser toggle, pause, resume, next, previous o select", http.StatusBadRequest)
		return
	}
	s.lastCycleAt = time.Now()
	writeCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	if configChanged {
		if err := s.persistConfig(writeCtx, s.config); err != nil {
			cancel()
			s.mu.Unlock()
			http.Error(w, "No se pudo guardar el estado de reproducción", http.StatusInternalServerError)
			return
		}
	}
	runtime := cloneWeatherRuntime(s.runtime)
	config := s.configViewLocked()
	if request.Action == "next" || request.Action == "previous" || request.Action == "select" {
		if err := s.persistRuntime(writeCtx, runtime); err != nil {
			cancel()
			s.mu.Unlock()
			http.Error(w, "No se pudo guardar la vista seleccionada", http.StatusInternalServerError)
			return
		}
	}
	cancel()
	s.mu.Unlock()
	s.broadcast("configUpdate", config)
	s.broadcast("viewChanged", map[string]int{"index": runtime.ActiveViewIndex})
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "config": config})
}

func (s *weatherService) run(stop <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.ctx = ctx
	s.lastCycleAt = time.Now()
	s.mu.Unlock()
	defer cancel()
	s.startRefresh(false)
	cycleTicker := time.NewTicker(250 * time.Millisecond)
	weatherTicker := time.NewTicker(weatherRefreshEvery)
	defer cycleTicker.Stop()
	defer weatherTicker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-cycleTicker.C:
			s.advanceViewIfDue(time.Now())
		case <-weatherTicker.C:
			s.startRefresh(false)
		}
	}
}

func (s *weatherService) advanceViewIfDue(now time.Time) {
	s.mu.Lock()
	if s.config.CyclePaused || now.Sub(s.lastCycleAt) < time.Duration(s.config.CycleDuration)*time.Second {
		s.mu.Unlock()
		return
	}
	queueLen := weatherViewCount(s.config.Locations)
	if queueLen == 0 {
		s.runtime.ActiveViewIndex = 0
		s.lastCycleAt = now
		s.mu.Unlock()
		return
	}
	s.runtime.ActiveViewIndex = (s.runtime.ActiveViewIndex + 1) % queueLen
	s.lastCycleAt = now
	index := s.runtime.ActiveViewIndex
	s.mu.Unlock()
	s.broadcast("viewChanged", map[string]int{"index": index})
}

func (s *weatherService) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		http.Error(w, "Se requiere text/event-stream", http.StatusNotAcceptable)
		return
	}
	typeName := r.URL.Query().Get("type")
	if typeName != "dashboard" && typeName != "overlay" && typeName != "preview" {
		http.Error(w, "El tipo de cliente debe ser dashboard, overlay o preview", http.StatusBadRequest)
		return
	}
	clientID := r.URL.Query().Get("clientId")
	if !weatherClientIDPattern.MatchString(clientID) {
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
	client := &weatherClient{id: clientID, typeName: typeName, events: make(chan weatherEvent, 32)}
	s.mu.Lock()
	s.clients[clientID] = client
	config := s.configViewLocked()
	runtime := cloneWeatherRuntime(s.runtime)
	connections := s.connectionStatusLocked()
	refreshing := s.refreshing
	s.mu.Unlock()
	defer s.removeClient(clientID, client)
	w.WriteHeader(http.StatusOK)
	if writeWeatherEvent(w, flusher, "configUpdate", config) != nil ||
		writeWeatherEvent(w, flusher, "weatherUpdate", map[string]any{"conditions": runtime.Conditions, "refreshing": refreshing, "lastAttempt": runtime.LastAttempt, "lastRefresh": runtime.LastRefresh, "lastError": runtime.LastError}) != nil ||
		writeWeatherEvent(w, flusher, "connectionStatus", connections) != nil {
		return
	}
	s.broadcast("connectionStatus", connections)
	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-client.events:
			if err := writeRawWeatherEvent(w, flusher, event); err != nil {
				return
			}
		case <-keepAlive.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *weatherService) removeClient(clientID string, client *weatherClient) {
	s.mu.Lock()
	if s.clients[clientID] != client {
		s.mu.Unlock()
		return
	}
	delete(s.clients, clientID)
	connections := s.connectionStatusLocked()
	s.mu.Unlock()
	s.broadcast("connectionStatus", connections)
}

func (s *weatherService) connectionStatusLocked() weatherConnectionStatus {
	var status weatherConnectionStatus
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

func (s *weatherService) broadcast(name string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	event := weatherEvent{name: name, payload: data}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, client := range s.clients {
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

func writeWeatherEvent(w http.ResponseWriter, flusher http.Flusher, name string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return writeRawWeatherEvent(w, flusher, weatherEvent{name: name, payload: data})
}

func writeRawWeatherEvent(w http.ResponseWriter, flusher http.Flusher, event weatherEvent) error {
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.name, event.payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
