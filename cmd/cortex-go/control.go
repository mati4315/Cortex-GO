package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//go:embed web/index.html
var webFiles embed.FS

type controlServer struct {
	manager  *processSupervisor
	shutdown func()
	client   *http.Client
	gitSync  *gitSyncManager
}

func newControlServer(manager *processSupervisor, shutdown func()) http.Handler {
	control := &controlServer{
		manager:  manager,
		shutdown: shutdown,
		client:   &http.Client{Timeout: 2500 * time.Millisecond},
		gitSync:  newGitSyncManager(discoverProjectRoot(), findGitExecutable()),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", control.handleHealth)
	mux.HandleFunc("GET /api/status", control.handleStatus)
	mux.HandleFunc("GET /api/events", control.proxy)
	mux.HandleFunc("POST /api/simulate/comment", control.proxy)
	mux.HandleFunc("POST /api/backup", control.proxy)
	mux.HandleFunc("POST /api/restore", control.proxy)
	mux.HandleFunc("POST /api/feeds/{id}/disconnect", control.proxy)
	mux.HandleFunc("POST /api/feeds/{id}/reconnect", control.proxy)
	mux.HandleFunc("POST /api/worker/restart", control.handleRestart)
	mux.HandleFunc("POST /api/shutdown", control.handleShutdown)
	mux.HandleFunc("GET /api/git/preview", control.handleGitPreview)
	mux.HandleFunc("POST /api/git/publish", control.handleGitPublish)
	mux.HandleFunc("GET /", control.handleIndex)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Cortex-Go-Control-Plane", "1")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !sameOrigin(r) {
			http.Error(w, "La solicitud del navegador no tiene el mismo origen que Cortex Go", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (c *controlServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"state": "running"})
}

func (c *controlServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	content, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "No se pudo cargar el panel", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (c *controlServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	request, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, c.manager.WorkerURL()+"/api/status", nil)
	response, err := c.client.Do(request)
	if err != nil {
		backendState := c.manager.Snapshot().State
		if backendState == "running" {
			backendState = "starting"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"supervisor":      c.manager.Snapshot(),
			"backend":         map[string]any{"state": backendState},
			"storage":         map[string]any{"state": "unavailable"},
			"obs":             map[string]any{"state": "unavailable", "label": "Estado pendiente"},
			"feeds":           []any{},
			"workerReachable": false,
		})
		return
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"supervisor": c.manager.Snapshot(), "workerReachable": false,
			"error": "El servidor interno envió un estado inválido",
		})
		return
	}
	payload["supervisor"] = c.manager.Snapshot()
	payload["workerReachable"] = response.StatusCode == http.StatusOK
	writeJSON(w, http.StatusOK, payload)
}

func (c *controlServer) proxy(w http.ResponseWriter, r *http.Request) {
	target := c.manager.WorkerURL() + r.URL.RequestURI()
	request, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		http.Error(w, "No se pudo preparar la acción para el servidor interno", http.StatusBadRequest)
		return
	}
	request.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	client := c.client
	if r.URL.Path == "/api/backup" || r.URL.Path == "/api/restore" {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	response, err := client.Do(request)
	if err != nil {
		http.Error(w, "El servidor interno está reiniciando; probá de nuevo en unos segundos", http.StatusServiceUnavailable)
		return
	}
	defer response.Body.Close()
	for _, header := range []string{"Content-Type"} {
		if value := response.Header.Get(header); value != "" {
			w.Header().Set(header, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
}

func (c *controlServer) handleRestart(w http.ResponseWriter, _ *http.Request) {
	c.manager.RequestRestart()
	writeJSON(w, http.StatusAccepted, map[string]string{"state": "restart_requested"})
}

func (c *controlServer) handleShutdown(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusAccepted, map[string]string{"state": "stopping"})
	go c.shutdown()
}

func (c *controlServer) handleGitPreview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	preview, err := c.gitSync.Preview(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (c *controlServer) handleGitPublish(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input gitPublishRequest
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "La confirmación de publicación no es válida", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "La confirmación de publicación contiene datos adicionales", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	result, err := c.gitSync.Publish(ctx, input)
	if err != nil {
		if result.State == "committed_not_pushed" {
			http.Error(w, result.Message+" Commit: "+result.Commit+". Error de Git: "+err.Error(), http.StatusBadGateway)
			return
		}
		status := http.StatusUnprocessableEntity
		if strings.Contains(err.Error(), "cambiaron desde la revisión") || strings.Contains(err.Error(), "lista de archivos a publicar cambió") {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // clientes locales de automatización pueden no enviar Origin
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if parsed.Scheme == "http" && strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	return parsed.Scheme == "https" &&
		strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") &&
		strings.EqualFold(parsed.Host, r.Header.Get("X-Forwarded-Host")) &&
		r.Header.Get("Tailscale-User-Login") != ""
}
