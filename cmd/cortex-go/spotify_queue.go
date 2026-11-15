package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type spotifyOperation struct {
	OperationID      string `json:"operationId"`
	EventID          int64  `json:"eventId"`
	ProgramSessionID string `json:"programSessionId"`
	TrackURI         string `json:"trackUri"`
	Status           string `json:"status"`
	Detail           string `json:"detail"`
	SentAt           string `json:"sentAt,omitempty"`
	RetryAt          string `json:"retryAt,omitempty"`
	AcceptedAt       string `json:"acceptedAt,omitempty"`
	Attempts         int    `json:"attempts"`
	ExpiresAt        string `json:"expiresAt"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

func (s *spotifyService) QueueTrackForEvent(ctx context.Context, eventID int64, trackURI string) (spotifyOperation, error) {
	if eventID < 1 || !validSpotifyTrackURI(trackURI) {
		return spotifyOperation{}, errors.New("el comentario o la pista Spotify no son válidos")
	}
	event, err := s.spotifyEventContext(ctx, eventID)
	if err != nil {
		return spotifyOperation{}, err
	}
	if strings.HasPrefix(event.sourceID, "test:") || event.programSessionID == "" {
		return spotifyOperation{}, errors.New("el pedido no pertenece a un comentario de un programa activo")
	}
	window := time.Duration(s.settingsSnapshot().RecentOrderMinutes) * time.Minute
	capturedAt, err := time.Parse(time.RFC3339Nano, event.capturedAt)
	if err != nil {
		return spotifyOperation{}, errors.New("la fecha del comentario no es válida")
	}
	expiresAt := capturedAt.Add(window).UTC().Format(time.RFC3339Nano)
	operationID := spotifyOperationID(event, trackURI)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return spotifyOperation{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO spotify_operations
		(operation_id,event_id,program_session_id,track_uri,status,expires_at,created_at,updated_at)
		VALUES(?,?,?,?,'pendiente',?,?,?)`, operationID, eventID, event.programSessionID, trackURI, expiresAt, now, now); err != nil {
		return spotifyOperation{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO spotify_operation_order(operation_id) VALUES(?)`, operationID); err != nil {
		return spotifyOperation{}, err
	}
	if err := tx.Commit(); err != nil {
		return spotifyOperation{}, err
	}
	operation, err := s.getSpotifyOperation(ctx, operationID)
	if err != nil {
		return spotifyOperation{}, err
	}
	if operation.Status != "pendiente" {
		s.notifySpotifyOperation(ctx, operation)
	}
	return operation, nil
}

type spotifyEventContext struct {
	sourceID         string
	externalID       string
	streamID         string
	programSessionID string
	capturedAt       string
}

func (s *spotifyService) spotifyEventContext(ctx context.Context, eventID int64) (spotifyEventContext, error) {
	var event spotifyEventContext
	err := s.store.db.QueryRowContext(ctx, `SELECT source_id,external_id,stream_id,program_session_id,captured_at FROM events WHERE id=?`, eventID).Scan(
		&event.sourceID, &event.externalID, &event.streamID, &event.programSessionID, &event.capturedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return spotifyEventContext{}, errors.New("no se encontró el comentario original")
	}
	if err != nil {
		return spotifyEventContext{}, err
	}
	return event, nil
}

func spotifyOperationID(event spotifyEventContext, trackURI string) string {
	identity := strings.Join([]string{event.sourceID, event.externalID, event.streamID, event.programSessionID, trackURI}, "\x00")
	if event.externalID == "" {
		identity += "\x00" + event.capturedAt
	}
	hash := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(hash[:])
}

func validSpotifyTrackURI(value string) bool {
	if !strings.HasPrefix(value, "spotify:track:") || len(value) != len("spotify:track:")+22 {
		return false
	}
	for _, char := range strings.TrimPrefix(value, "spotify:track:") {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func (s *spotifyService) getSpotifyOperation(ctx context.Context, operationID string) (spotifyOperation, error) {
	var operation spotifyOperation
	err := s.store.db.QueryRowContext(ctx, `SELECT operation_id,event_id,program_session_id,track_uri,status,detail,sent_at,retry_at,accepted_at,attempts,expires_at,created_at,updated_at
		FROM spotify_operations WHERE operation_id=?`, operationID).Scan(
		&operation.OperationID, &operation.EventID, &operation.ProgramSessionID, &operation.TrackURI, &operation.Status,
		&operation.Detail, &operation.SentAt, &operation.RetryAt, &operation.AcceptedAt, &operation.Attempts,
		&operation.ExpiresAt, &operation.CreatedAt, &operation.UpdatedAt)
	return operation, err
}

func (s *spotifyService) processSpotifyOperation(ctx context.Context, operation spotifyOperation) (spotifyOperation, error) {
	if operation.Status != "pendiente" {
		return operation, nil
	}
	if operation.SentAt != "" {
		return s.updateSpotifyOperation(ctx, operation, "incierto", "El proceso se reinició después del envío; se requiere consultar Spotify. No se reenvía automáticamente.", operation.SentAt, "", "")
	}
	now := time.Now().UTC()
	if now.After(parseTimestamp(operation.ExpiresAt)) {
		return s.updateSpotifyOperation(ctx, operation, "vencido", "El pedido superó la ventana de recuperación de su programa y no se envió.", "", "", "")
	}
	var activeSession string
	err := s.store.db.QueryRowContext(ctx, `SELECT id FROM program_sessions WHERE status='active' LIMIT 1`).Scan(&activeSession)
	if err != nil || activeSession != operation.ProgramSessionID {
		return s.updateSpotifyOperation(ctx, operation, "vencido", "El programa vinculado ya no está activo; el pedido no se envió.", "", "", "")
	}
	if !s.settingsSnapshot().LiveQueueEnabled || s.ruloGate == nil {
		return s.updateSpotifyOperation(ctx, operation, "cancelado", "La cola Spotify está deshabilitada; no se envió el pedido.", "", "", "")
	}
	if operation.RetryAt != "" && now.Before(parseTimestamp(operation.RetryAt)) {
		return operation, nil
	}
	if !s.ruloGate() {
		return s.deferSpotifyOperation(ctx, operation, "Rulo está pausado; el pedido solo se retomará si sigue vigente en este programa.", 10*time.Second)
	}
	if !s.hasRefreshToken() {
		return s.updateSpotifyOperation(ctx, operation, "rechazado", "Spotify no está conectado; no se envió el pedido.", "", "", "")
	}
	var exists int
	err = s.store.db.QueryRowContext(ctx, `SELECT 1 FROM events WHERE id=? AND program_session_id=? AND source_id NOT LIKE 'test:%'`, operation.EventID, operation.ProgramSessionID).Scan(&exists)
	if err != nil {
		return s.updateSpotifyOperation(ctx, operation, "cancelado", "El comentario original ya no está activo; no se envió el pedido.", "", "", "")
	}
	if operation.Attempts >= 3 {
		return s.updateSpotifyOperation(ctx, operation, "fallido", "Se agotaron los intentos seguros previos al envío.", "", "", "")
	}
	playback, err := s.fetchCurrentPlayback(ctx)
	if err != nil {
		return s.deferSpotifyOperation(ctx, operation, "No se pudo verificar el dispositivo activo de Spotify.", 20*time.Second)
	}
	if playback == nil || playback.Device.ID == "" || !playback.Device.IsActive {
		return s.deferSpotifyOperation(ctx, operation, "Esperando un dispositivo activo de Spotify.", 20*time.Second)
	}
	queueBody, queueStatus, err := s.spotifyGet(ctx, "https://api.spotify.com/v1/me/player/queue")
	if err != nil {
		return s.deferSpotifyOperation(ctx, operation, "No se pudo consultar la cola de Spotify.", 20*time.Second)
	}
	if queueStatus == http.StatusTooManyRequests {
		return s.deferSpotifyOperation(ctx, operation, "Spotify limitó la consulta de la cola.", 30*time.Second)
	}
	if queueStatus != http.StatusOK && queueStatus != http.StatusNoContent {
		return s.updateSpotifyOperation(ctx, operation, "rechazado", spotifyStatusError(queueStatus).Error(), "", "", "")
	}
	var queue struct {
		CurrentlyPlaying *spotifyCurrentTrack  `json:"currently_playing"`
		Items            []spotifyCurrentTrack `json:"queue"`
	}
	if queueStatus == http.StatusOK {
		if err := decodeSpotifyResponse(strings.NewReader(string(queueBody)), &queue); err != nil || len(queue.Items) > 100 {
			return s.deferSpotifyOperation(ctx, operation, "Spotify devolvió una cola no válida.", 30*time.Second)
		}
	}
	if playback.Item != nil && playback.Item.URI == operation.TrackURI && playback.IsPlaying {
		return s.updateSpotifyOperation(ctx, operation, "completado", "La pista ya estaba sonando en el dispositivo activo; no se duplicó.", "", "", now.Format(time.RFC3339Nano))
	}
	if queue.CurrentlyPlaying != nil && queue.CurrentlyPlaying.URI == operation.TrackURI {
		return s.updateSpotifyOperation(ctx, operation, "aceptado", "La pista ya estaba en el reproductor; no se agregó otra copia.", "", "", now.Format(time.RFC3339Nano))
	}
	for _, item := range queue.Items {
		if item.URI == operation.TrackURI {
			return s.updateSpotifyOperation(ctx, operation, "aceptado", "La pista ya estaba en la cola; no se agregó otra copia.", "", "", now.Format(time.RFC3339Nano))
		}
	}
	s.actionMu.RLock()
	token, tokenErr := s.getAccessToken(ctx)
	s.actionMu.RUnlock()
	if tokenErr != nil {
		return s.deferSpotifyOperation(ctx, operation, "No se pudo renovar la autorización antes del envío.", 30*time.Second)
	}
	s.actionMu.RLock()
	defer s.actionMu.RUnlock()
	if !s.settingsSnapshot().LiveQueueEnabled || s.ruloGate == nil || !s.hasRefreshToken() {
		return s.updateSpotifyOperation(ctx, operation, "cancelado", "La cola Spotify se deshabilitó antes del envío; el pedido no se agregó.", "", "", "")
	}
	if !s.ruloGate() {
		return s.deferSpotifyOperation(ctx, operation, "Rulo se pausó antes del envío; el pedido solo se retomará si sigue vigente en este programa.", 10*time.Second)
	}
	var currentSession string
	if err := s.store.db.QueryRowContext(ctx, `SELECT id FROM program_sessions WHERE status='active' LIMIT 1`).Scan(&currentSession); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return s.updateSpotifyOperation(ctx, operation, "vencido", "El programa terminó antes del envío; no se agregó el pedido.", "", "", "")
		}
		return s.deferSpotifyOperation(ctx, operation, "No se pudo confirmar el programa activo antes del envío.", 20*time.Second)
	}
	if currentSession != operation.ProgramSessionID {
		return s.updateSpotifyOperation(ctx, operation, "vencido", "El programa terminó antes del envío; no se agregó el pedido.", "", "", "")
	}
	sentAt := now.Format(time.RFC3339Nano)
	operation, err = s.updateSpotifyOperation(ctx, operation, "pendiente", "Envío preparado; esperando respuesta confirmada de Spotify.", sentAt, "", "")
	if err != nil {
		return spotifyOperation{}, err
	}
	operation.Attempts++
	if _, err := s.store.db.ExecContext(ctx, `UPDATE spotify_operations SET attempts=?,updated_at=? WHERE operation_id=?`, operation.Attempts, sentAt, operation.OperationID); err != nil {
		return spotifyOperation{}, err
	}
	status, retryAfter, sendErr := s.sendSpotifyQueueItem(ctx, operation.TrackURI, playback.Device.ID, token)
	if sendErr != nil {
		return s.updateSpotifyOperation(ctx, operation, "incierto", "La llamada pudo llegar a Spotify, pero no se confirmó el resultado. No se reenvía automáticamente.", sentAt, "", "")
	}
	switch {
	case status == http.StatusNoContent:
		return s.updateSpotifyOperation(ctx, operation, "aceptado", "Spotify confirmó que recibió el pedido en la cola; esto no confirma reproducción.", sentAt, "", time.Now().UTC().Format(time.RFC3339Nano))
	case status == http.StatusTooManyRequests:
		operation.Attempts--
		if operation.Attempts < 0 {
			operation.Attempts = 0
		}
		_, _ = s.store.db.ExecContext(ctx, `UPDATE spotify_operations SET attempts=? WHERE operation_id=?`, operation.Attempts, operation.OperationID)
		return s.deferSpotifyOperation(ctx, operation, "Spotify limitó el pedido antes de agregarlo.", retryAfter)
	case status >= 500:
		return s.updateSpotifyOperation(ctx, operation, "incierto", "Spotify respondió con un error de servidor después del envío; no se reenvía automáticamente.", sentAt, "", "")
	default:
		detail := "Spotify rechazó el pedido; no se agregó a la cola."
		if status == http.StatusForbidden {
			detail = "Spotify rechazó el pedido; verifica Premium, el dispositivo y los permisos solicitados."
		}
		return s.updateSpotifyOperation(ctx, operation, "rechazado", detail, sentAt, "", "")
	}
}

func (s *spotifyService) updateSpotifyOperation(ctx context.Context, operation spotifyOperation, status, detail, sentAt, retryAt, acceptedAt string) (spotifyOperation, error) {
	if len([]rune(detail)) > 240 {
		detail = string([]rune(detail)[:240])
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.store.db.ExecContext(ctx, `UPDATE spotify_operations SET status=?,detail=?,sent_at=?,retry_at=?,accepted_at=?,updated_at=? WHERE operation_id=?`,
		status, detail, sentAt, retryAt, acceptedAt, now, operation.OperationID)
	if err != nil {
		return spotifyOperation{}, err
	}
	updated, err := s.getSpotifyOperation(ctx, operation.OperationID)
	if err != nil {
		return spotifyOperation{}, err
	}
	if operation.Status != updated.Status || operation.Detail != updated.Detail {
		s.notifySpotifyOperation(ctx, updated)
	}
	return updated, nil
}

func (s *spotifyService) deferSpotifyOperation(ctx context.Context, operation spotifyOperation, detail string, delay time.Duration) (spotifyOperation, error) {
	if delay < 5*time.Second {
		delay = 5 * time.Second
	}
	if delay > 2*time.Minute {
		delay = 2 * time.Minute
	}
	return s.updateSpotifyOperation(ctx, operation, "pendiente", detail, "", time.Now().UTC().Add(delay).Format(time.RFC3339Nano), "")
}

func (s *spotifyService) sendSpotifyQueueItem(ctx context.Context, trackURI, deviceID, token string) (int, time.Duration, error) {
	endpoint, _ := url.Parse("https://api.spotify.com/v1/me/player/queue")
	query := endpoint.Query()
	query.Set("uri", trackURI)
	query.Set("device_id", deviceID)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return 0, 0, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return 0, 0, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		seconds, _ := strconv.Atoi(response.Header.Get("Retry-After"))
		return response.StatusCode, time.Duration(seconds) * time.Second, nil
	}
	return response.StatusCode, 0, nil
}

func (s *spotifyService) notifySpotifyOperation(ctx context.Context, operation spotifyOperation) {
	if s.resultSink == nil {
		return
	}
	if operation.Status == "pendiente" && operation.RetryAt == "" {
		return
	}
	signal, message := "warning", ""
	switch operation.Status {
	case "aceptado":
		signal, message = "success", "Spotify confirmó que el pedido quedó en la cola; todavía no significa que esté sonando."
	case "completado":
		signal, message = "success", "Spotify confirmó que la pista está sonando."
	case "pendiente":
		signal, message = "warning", "El pedido musical sigue pendiente: "+operation.Detail
	case "rechazado", "fallido", "incierto", "vencido", "cancelado":
		signal, message = "warning", "Resultado Spotify: "+operation.Detail
	default:
		return
	}
	if len([]rune(message)) > 300 {
		message = string([]rune(message)[:300])
	}
	_ = s.resultSink(ctx, operation.EventID, signal, message)
}

func (s *spotifyService) fetchSpotifyQueue(ctx context.Context) ([]spotifyCurrentTrack, int, error) {
	body, status, err := s.spotifyGet(ctx, "https://api.spotify.com/v1/me/player/queue")
	if err != nil {
		return nil, status, err
	}
	if status == http.StatusNoContent {
		return nil, status, nil
	}
	if status != http.StatusOK {
		return nil, status, spotifyStatusError(status)
	}
	var result struct {
		Queue []spotifyCurrentTrack `json:"queue"`
	}
	if err := decodeSpotifyResponse(strings.NewReader(string(body)), &result); err != nil || len(result.Queue) > 100 {
		return nil, status, errors.New("Spotify devolvió una cola no válida")
	}
	return result.Queue, status, nil
}

func (s *spotifyService) recoverInterruptedSpotifyOperations(ctx context.Context) error {
	rows, err := s.store.db.QueryContext(ctx, `SELECT operation_id FROM spotify_operations WHERE status='pendiente' AND sent_at<>'' ORDER BY created_at LIMIT 100`)
	if err != nil {
		return err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		operation, err := s.getSpotifyOperation(ctx, id)
		if err != nil {
			continue
		}
		if _, err := s.updateSpotifyOperation(ctx, operation, "incierto", "El proceso se reinició después del envío; se requiere consultar Spotify. No se reenvía automáticamente.", operation.SentAt, "", ""); err != nil {
			return err
		}
	}
	return nil
}

func (s *spotifyService) processPendingSpotifyOperations(ctx context.Context) error {
	rows, err := s.store.db.QueryContext(ctx, `SELECT o.operation_id FROM spotify_operations o
		JOIN spotify_operation_order q ON q.operation_id=o.operation_id
		WHERE o.status='pendiente' AND o.sent_at='' ORDER BY q.queue_sequence LIMIT 1`)
	if err != nil {
		return err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(ids) == 0 {
		return nil
	}
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	operation, err := s.getSpotifyOperation(ctx, ids[0])
	if err != nil {
		return err
	}
	_, err = s.processSpotifyOperation(ctx, operation)
	return err
}

func (s *spotifyService) reconcileAcceptedSpotifyOperations(ctx context.Context) error {
	cutoff := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano)
	var count int
	if err := s.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM spotify_operations WHERE status='aceptado' AND accepted_at>=?`, cutoff).Scan(&count); err != nil || count == 0 {
		return err
	}
	playback, err := s.fetchCurrentPlayback(ctx)
	if err != nil || playback == nil || playback.Item == nil || !playback.IsPlaying {
		return err
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT operation_id FROM spotify_operations WHERE status='aceptado' AND track_uri=? AND accepted_at>=? ORDER BY accepted_at LIMIT 20`, playback.Item.URI, cutoff)
	if err != nil {
		return err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		operation, err := s.getSpotifyOperation(ctx, id)
		if err != nil {
			continue
		}
		acceptedAt := parseTimestamp(operation.AcceptedAt)
		if playback.Timestamp > 0 && time.UnixMilli(playback.Timestamp).Before(acceptedAt.Add(-2*time.Second)) {
			continue
		}
		_, err = s.updateSpotifyOperation(ctx, operation, "completado", "La reproducción actual de Spotify coincide con la pista solicitada.", operation.SentAt, "", operation.AcceptedAt)
		return err
	}
	return nil
}

func (s *spotifyService) handleOperations(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var active string
	err := s.store.db.QueryRowContext(ctx, `SELECT id FROM program_sessions WHERE status='active' LIMIT 1`).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"operations": []spotifyOperation{}})
		return
	}
	if err != nil {
		http.Error(w, "No se pudieron leer los pedidos de Spotify", http.StatusServiceUnavailable)
		return
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT o.operation_id,o.event_id,o.program_session_id,o.track_uri,o.status,o.detail,o.sent_at,o.retry_at,o.accepted_at,o.attempts,o.expires_at,o.created_at,o.updated_at
		FROM spotify_operations o JOIN spotify_operation_order q ON q.operation_id=o.operation_id
		WHERE o.program_session_id=? ORDER BY q.queue_sequence DESC LIMIT 30`, active)
	if err != nil {
		http.Error(w, "No se pudieron leer los pedidos de Spotify", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	operations := make([]spotifyOperation, 0)
	for rows.Next() {
		var operation spotifyOperation
		if err := rows.Scan(&operation.OperationID, &operation.EventID, &operation.ProgramSessionID, &operation.TrackURI, &operation.Status,
			&operation.Detail, &operation.SentAt, &operation.RetryAt, &operation.AcceptedAt, &operation.Attempts,
			&operation.ExpiresAt, &operation.CreatedAt, &operation.UpdatedAt); err != nil {
			http.Error(w, "No se pudieron leer los pedidos de Spotify", http.StatusServiceUnavailable)
			return
		}
		operations = append(operations, operation)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "No se pudieron leer los pedidos de Spotify", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"operations": operations})
}

func (s *spotifyService) handleSearch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		Query string `json:"query"`
	}
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "La búsqueda de Spotify no es válida", http.StatusBadRequest)
		return
	}
	input.Query = strings.TrimSpace(input.Query)
	if len([]rune(input.Query)) < 2 || len([]rune(input.Query)) > 160 {
		http.Error(w, "Escribe entre 2 y 160 caracteres para buscar", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tracks, err := s.searchSpotifyTracks(ctx, input.Query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tracks": tracks})
}

func (s *spotifyService) searchSpotifyTracks(ctx context.Context, text string) ([]spotifyCurrentTrack, error) {
	endpoint, _ := url.Parse("https://api.spotify.com/v1/search")
	query := endpoint.Query()
	query.Set("q", text)
	query.Set("type", "track")
	query.Set("limit", "5")
	endpoint.RawQuery = query.Encode()
	body, status, err := s.spotifyGet(ctx, endpoint.String())
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, spotifyStatusError(status)
	}
	var result struct {
		Tracks struct {
			Items []spotifyCurrentTrack `json:"items"`
		} `json:"tracks"`
	}
	if err := decodeSpotifyResponse(strings.NewReader(string(body)), &result); err != nil || len(result.Tracks.Items) > 5 {
		return nil, errors.New("Spotify devolvió una búsqueda no válida")
	}
	return result.Tracks.Items, nil
}

func chooseExactSpotifyTrack(query string, tracks []spotifyCurrentTrack) (spotifyCurrentTrack, bool) {
	title, artist := splitSpotifyTitleArtist(query)
	if title == "" || artist == "" {
		return spotifyCurrentTrack{}, false
	}
	matches := make([]spotifyCurrentTrack, 0, len(tracks))
	artists := make(map[string]bool)
	for _, track := range tracks {
		if normalizeSpotifyMatchText(track.Name) != title || len(track.Artists) == 0 || (track.IsPlayable != nil && !*track.IsPlayable) || !validSpotifyTrackURI(track.URI) {
			continue
		}
		matchedArtist := false
		for _, currentArtist := range track.Artists {
			name := normalizeSpotifyMatchText(currentArtist.Name)
			if artist == "" || name == artist {
				matchedArtist = true
				artists[name] = true
			}
		}
		if matchedArtist {
			matches = append(matches, track)
		}
	}
	if len(matches) == 0 || len(artists) > 1 {
		return spotifyCurrentTrack{}, false
	}
	uniqueURIs := make(map[string]bool)
	for _, match := range matches {
		uniqueURIs[match.URI] = true
	}
	if len(uniqueURIs) != 1 {
		return spotifyCurrentTrack{}, false
	}
	return matches[0], true
}

func chooseSpotifyArtistTrack(artist string, tracks []spotifyCurrentTrack) (spotifyCurrentTrack, bool) {
	artist = normalizeSpotifyMatchText(artist)
	if artist == "" {
		return spotifyCurrentTrack{}, false
	}
	for _, track := range tracks {
		if track.Name == "" || len(track.Artists) == 0 || (track.IsPlayable != nil && !*track.IsPlayable) || !validSpotifyTrackURI(track.URI) {
			continue
		}
		for _, trackArtist := range track.Artists {
			if normalizeSpotifyMatchText(trackArtist.Name) == artist {
				return track, true
			}
		}
	}
	return spotifyCurrentTrack{}, false
}

func hasSpotifyTitleAndArtist(query string) bool {
	title, artist := splitSpotifyTitleArtist(query)
	return title != "" && artist != ""
}

func splitSpotifyTitleArtist(query string) (string, string) {
	query = normalizeSpotifyMatchText(query)
	split := strings.LastIndex(query, " de ")
	if split <= 0 || split+4 >= len(query) {
		return "", ""
	}
	return strings.TrimSpace(query[:split]), strings.TrimSpace(query[split+4:])
}

func normalizeSpotifyMatchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n").Replace(value)
	var result strings.Builder
	space := true
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			result.WriteRune(char)
			space = false
		} else if !space {
			result.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(result.String())
}

func (s *spotifyService) settingsSnapshot() spotifySettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *spotifyService) setRuloIntegration(gate func() bool, resultSink func(context.Context, int64, string, string) error) {
	s.ruloGate = gate
	s.resultSink = resultSink
}

func (s *spotifyService) run(stop <-chan struct{}) {
	lock := func() {}
	unlock := func() {}
	if s.restoreMu != nil {
		lock = s.restoreMu.RLock
		unlock = s.restoreMu.RUnlock
	}
	lock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_ = s.recoverInterruptedSpotifyOperations(ctx)
	cancel()
	unlock()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastReconcile := time.Now()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			lock()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = s.processPendingSpotifyOperations(ctx)
			if time.Since(lastReconcile) >= 15*time.Second {
				_ = s.reconcileAcceptedSpotifyOperations(ctx)
				lastReconcile = time.Now()
			}
			cancel()
			unlock()
		}
	}
}

func (s *spotifyService) beginRuloQueueForEvent(ctx context.Context, eventID int64, trackURI string) (spotifyOperation, error) {
	return s.QueueTrackForEvent(ctx, eventID, trackURI)
}
