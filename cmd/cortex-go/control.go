package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

//go:embed web
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
	mux.HandleFunc("GET /api/settings/startup", control.handleStartupSettings)
	mux.HandleFunc("POST /api/settings/startup", control.handleStartupSettings)
	mux.HandleFunc("GET /api/events", control.proxy)
	mux.HandleFunc("GET /api/events/statistics", control.proxy)
	mux.HandleFunc("GET /api/events/export.csv", control.proxy)
	mux.HandleFunc("GET /api/modules/facebook/events", control.proxy)
	mux.HandleFunc("GET /api/modules/facebook/featured/events", control.proxy)
	mux.HandleFunc("POST /api/modules/facebook/featured", control.proxy)
	mux.HandleFunc("POST /api/events", control.proxy)
	mux.HandleFunc("DELETE /api/events", control.proxy)
	mux.HandleFunc("DELETE /api/events/{id}", control.proxy)
	mux.HandleFunc("GET /api/event-archives", control.proxy)
	mux.HandleFunc("POST /api/event-archives", control.proxy)
	mux.HandleFunc("GET /api/event-archives/{id}", control.proxy)
	mux.HandleFunc("DELETE /api/event-archives/{id}", control.proxy)
	mux.HandleFunc("POST /api/simulate/comment", control.proxy)
	mux.HandleFunc("GET /api/program/session", control.proxy)
	mux.HandleFunc("POST /api/program/session", control.proxy)
	mux.HandleFunc("DELETE /api/program/session", control.proxy)
	mux.HandleFunc("GET /api/modules/rulo/", control.proxy)
	mux.HandleFunc("POST /api/modules/rulo/", control.proxy)
	mux.HandleFunc("PUT /api/modules/rulo/", control.proxy)
	mux.HandleFunc("PATCH /api/modules/rulo/", control.proxy)
	mux.HandleFunc("DELETE /api/modules/rulo/", control.proxy)
	mux.HandleFunc("GET /api/modules/spotify/", control.proxy)
	mux.HandleFunc("PUT /api/modules/spotify/", control.proxy)
	mux.HandleFunc("POST /api/modules/spotify/", control.proxy)
	mux.HandleFunc("POST /api/backup", control.proxy)
	mux.HandleFunc("GET /api/backups", control.proxy)
	mux.HandleFunc("DELETE /api/backups", control.proxy)
	mux.HandleFunc("DELETE /api/backups/{name}", control.proxy)
	mux.HandleFunc("POST /api/restore", control.proxy)
	mux.HandleFunc("POST /api/feeds/{id}/disconnect", control.proxy)
	mux.HandleFunc("POST /api/feeds/{id}/reconnect", control.proxy)
	mux.HandleFunc("POST /api/feeds/{id}/status", control.proxy)
	mux.HandleFunc("POST /api/worker/restart", control.handleRestart)
	mux.HandleFunc("POST /api/shutdown", control.handleShutdown)
	mux.HandleFunc("GET /api/git/preview", control.handleGitPreview)
	mux.HandleFunc("POST /api/git/publish", control.handleGitPublish)
	mux.HandleFunc("GET /api/modules/welcome/", control.proxy)
	mux.HandleFunc("PUT /api/modules/welcome/", control.proxy)
	mux.HandleFunc("POST /api/modules/welcome/", control.proxy)
	mux.HandleFunc("GET /api/modules/lights/", control.proxy)
	mux.HandleFunc("PUT /api/modules/lights/", control.proxy)
	mux.HandleFunc("POST /api/modules/lights/", control.proxy)
	mux.HandleFunc("GET /api/modules/weather/", control.proxy)
	mux.HandleFunc("PUT /api/modules/weather/", control.proxy)
	mux.HandleFunc("POST /api/modules/weather/", control.proxy)
	mux.HandleFunc("GET /modules/bienvenidos/admin", func(w http.ResponseWriter, r *http.Request) {
		serveWelcomePage(w, r, "admin.html")
	})
	mux.HandleFunc("GET /modules/bienvenidos/starting-soon", func(w http.ResponseWriter, r *http.Request) {
		serveWelcomePage(w, r, "starting-soon.html")
	})
	mux.HandleFunc("GET /modules/bienvenidos/banner", func(w http.ResponseWriter, r *http.Request) {
		serveWelcomePage(w, r, "banner.html")
	})
	mux.HandleFunc("GET /modules/bienvenidos/{resource...}", control.serveWelcomeAsset)
	mux.HandleFunc("GET /modules/luces/admin", func(w http.ResponseWriter, r *http.Request) {
		serveLightsPage(w, r, "dashboard.html")
	})
	mux.HandleFunc("GET /modules/luces/overlay", func(w http.ResponseWriter, r *http.Request) {
		serveLightsPage(w, r, "overlay.html")
	})
	mux.HandleFunc("GET /modules/luces/{resource...}", control.serveLightsAsset)
	mux.HandleFunc("GET /modules/clima/admin", func(w http.ResponseWriter, r *http.Request) {
		serveWeatherPage(w, r, "dashboard.html")
	})
	mux.HandleFunc("GET /modules/clima/overlay", func(w http.ResponseWriter, r *http.Request) {
		serveWeatherPage(w, r, "overlay.html")
	})
	mux.HandleFunc("GET /modules/clima/{resource...}", control.serveWeatherAsset)
	mux.HandleFunc("GET /modules/facebook/history", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookPage(w, r)
	})
	mux.HandleFunc("GET /modules/facebook/overlay", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookOverlayPage(w, r)
	})
	mux.HandleFunc("GET /modules/facebook/featured", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookFeaturedPage(w, r)
	})
	mux.HandleFunc("GET /modules/facebook/{resource...}", control.serveFacebookAsset)
	mux.HandleFunc("GET /modules/rulo", func(w http.ResponseWriter, r *http.Request) {
		serveRuloPage(w, r)
	})
	mux.HandleFunc("GET /modules/rulo/presentation", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookWebFile(w, r, "web/rulo/presentation.html")
	})
	mux.HandleFunc("GET /modules/rulo/overlay", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookWebFile(w, r, "web/rulo/unified-overlay.html")
	})
	mux.HandleFunc("GET /modules/rulo/unified-overlay", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookWebFile(w, r, "web/rulo/unified-overlay.html")
	})
	mux.HandleFunc("GET /modules/spotify", func(w http.ResponseWriter, r *http.Request) {
		serveSpotifyPage(w, r)
	})
	mux.HandleFunc("GET /modules/spotify/{resource...}", control.serveSpotifyAsset)
	mux.HandleFunc("GET /rulo-chat-historial.html", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookPage(w, r)
	})
	mux.HandleFunc("GET /rulo-chat-overlay.html", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookOverlayPage(w, r)
	})
	mux.HandleFunc("GET /rulo-featured.html", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookFeaturedPage(w, r)
	})
	mux.HandleFunc("GET /rulo-unified-overlay.html", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookWebFile(w, r, "web/rulo/unified-overlay.html")
	})
	mux.HandleFunc("GET /rulo-unified-dashboard.html", func(w http.ResponseWriter, r *http.Request) {
		serveFacebookWebFile(w, r, "web/rulo/presentation.html")
	})
	mux.HandleFunc("GET /manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		serveWebAsset(w, r, "web/manifest.webmanifest", "application/manifest+json; charset=utf-8", "no-cache")
	})
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		serveWebAsset(w, r, "web/sw.js", "text/javascript; charset=utf-8", "no-cache")
	})
	mux.HandleFunc("GET /icons/icon-192.png", func(w http.ResponseWriter, r *http.Request) {
		serveWebAsset(w, r, "web/icons/icon-192.png", "image/png", "public, max-age=86400")
	})
	mux.HandleFunc("GET /icons/icon-512.png", func(w http.ResponseWriter, r *http.Request) {
		serveWebAsset(w, r, "web/icons/icon-512.png", "image/png", "public, max-age=86400")
	})
	mux.HandleFunc("GET /", control.handleIndex)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Cortex-Go-Control-Plane", "1")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob: https://i.scdn.co; media-src 'self' blob:; frame-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self' https://lrclib.net; object-src 'none'; base-uri 'none'; frame-ancestors 'self'")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !sameOrigin(r) {
			http.Error(w, "La solicitud del navegador no tiene el mismo origen que Cortex Go", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func serveWelcomePage(w http.ResponseWriter, r *http.Request, file string) {
	serveWelcomeWebFile(w, r, "web/welcome/"+file)
}

func (c *controlServer) serveWelcomeAsset(w http.ResponseWriter, r *http.Request) {
	resource := path.Clean("/" + r.PathValue("resource"))
	resource = strings.TrimPrefix(resource, "/")
	if resource == "" || resource == "." || strings.HasPrefix(resource, "../") || strings.Contains(resource, "\\") {
		http.NotFound(w, r)
		return
	}
	if !strings.HasPrefix(resource, "css/") && !strings.HasPrefix(resource, "js/") && !strings.HasPrefix(resource, "images/") && !strings.HasPrefix(resource, "fonts/") {
		http.NotFound(w, r)
		return
	}
	serveWelcomeWebFile(w, r, "web/welcome/"+resource)
}

func serveLightsPage(w http.ResponseWriter, r *http.Request, file string) {
	serveLightsWebFile(w, r, "web/lights/"+file)
}

func (c *controlServer) serveLightsAsset(w http.ResponseWriter, r *http.Request) {
	resource := path.Clean("/" + r.PathValue("resource"))
	resource = strings.TrimPrefix(resource, "/")
	if resource == "" || resource == "." || strings.HasPrefix(resource, "../") || strings.Contains(resource, "\\") ||
		(!strings.HasPrefix(resource, "js/") && !strings.HasPrefix(resource, "css/")) {
		http.NotFound(w, r)
		return
	}
	serveLightsWebFile(w, r, "web/lights/"+resource)
}

func serveWeatherPage(w http.ResponseWriter, r *http.Request, file string) {
	serveWeatherWebFile(w, r, "web/weather/"+file)
}

func (c *controlServer) serveWeatherAsset(w http.ResponseWriter, r *http.Request) {
	resource := path.Clean("/" + r.PathValue("resource"))
	resource = strings.TrimPrefix(resource, "/")
	if resource == "" || resource == "." || strings.HasPrefix(resource, "../") || strings.Contains(resource, "\\") ||
		(!strings.HasPrefix(resource, "js/") && !strings.HasPrefix(resource, "css/") && !strings.HasPrefix(resource, "images/flags/")) {
		http.NotFound(w, r)
		return
	}
	serveWeatherWebFile(w, r, "web/weather/"+resource)
}

func serveFacebookPage(w http.ResponseWriter, r *http.Request) {
	serveFacebookWebFile(w, r, "web/facebook/history.html")
}

func serveRuloPage(w http.ResponseWriter, r *http.Request) {
	content, err := webFiles.ReadFile("web/rulo/lab.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(content)
	}
}

func serveSpotifyPage(w http.ResponseWriter, r *http.Request) {
	serveFacebookWebFile(w, r, "web/spotify/dashboard.html")
}

func (c *controlServer) serveSpotifyAsset(w http.ResponseWriter, r *http.Request) {
	resource := path.Clean("/" + r.PathValue("resource"))
	resource = strings.TrimPrefix(resource, "/")
	if resource == "" || resource == "." || strings.HasPrefix(resource, "../") || strings.Contains(resource, "\\") ||
		(!strings.HasPrefix(resource, "js/") && !strings.HasPrefix(resource, "css/")) {
		http.NotFound(w, r)
		return
	}
	serveFacebookWebFile(w, r, "web/spotify/"+resource)
}

func serveFacebookOverlayPage(w http.ResponseWriter, r *http.Request) {
	serveFacebookWebFile(w, r, "web/facebook/overlay.html")
}

func serveFacebookFeaturedPage(w http.ResponseWriter, r *http.Request) {
	serveFacebookWebFile(w, r, "web/facebook/featured.html")
}

func (c *controlServer) serveFacebookAsset(w http.ResponseWriter, r *http.Request) {
	resource := path.Clean("/" + r.PathValue("resource"))
	resource = strings.TrimPrefix(resource, "/")
	if resource == "" || resource == "." || strings.HasPrefix(resource, "../") || strings.Contains(resource, "\\") ||
		(!strings.HasPrefix(resource, "css/") && !strings.HasPrefix(resource, "js/") && resource != "assets/kangaroo.svg") {
		http.NotFound(w, r)
		return
	}
	serveFacebookWebFile(w, r, "web/facebook/"+resource)
}

func serveFacebookWebFile(w http.ResponseWriter, r *http.Request, file string) {
	content, err := webFiles.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(file))
	if contentType == "" {
		switch path.Ext(file) {
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".js":
			contentType = "text/javascript; charset=utf-8"
		case ".svg":
			contentType = "image/svg+xml"
		default:
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(content)
	}
}

func serveWeatherWebFile(w http.ResponseWriter, r *http.Request, file string) {
	content, err := webFiles.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(file))
	if contentType == "" {
		switch path.Ext(file) {
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".js":
			contentType = "text/javascript; charset=utf-8"
		case ".woff2":
			contentType = "font/woff2"
		case ".svg":
			contentType = "image/svg+xml"
		default:
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(content)
	}
}

func serveLightsWebFile(w http.ResponseWriter, r *http.Request, file string) {
	content, err := webFiles.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(file))
	if contentType == "" {
		switch path.Ext(file) {
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".js":
			contentType = "text/javascript; charset=utf-8"
		default:
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(content)
	}
}

func serveWelcomeWebFile(w http.ResponseWriter, r *http.Request, file string) {
	content, err := webFiles.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(file))
	if contentType == "" {
		switch path.Ext(file) {
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".js":
			contentType = "text/javascript; charset=utf-8"
		case ".woff2":
			contentType = "font/woff2"
		default:
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(content)
	}
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

func serveWebAsset(w http.ResponseWriter, r *http.Request, file, contentType, cacheControl string) {
	content, err := webFiles.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(content)
	}
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
	for _, header := range []string{"Content-Type", "Accept", "Range", "If-Range", "If-None-Match", "If-Modified-Since", "Last-Event-ID"} {
		if value := r.Header.Get(header); value != "" {
			request.Header.Set(header, value)
		}
	}
	client := c.client
	if r.URL.Path == "/api/backup" || r.URL.Path == "/api/restore" || r.URL.Path == "/api/events/export.csv" || strings.HasPrefix(r.URL.Path, "/api/event-archives") || strings.HasPrefix(r.URL.Path, "/api/modules/welcome/upload/") {
		client = &http.Client{Timeout: 2 * time.Minute}
	} else if r.URL.Path == "/api/modules/spotify/callback" || r.URL.Path == "/api/modules/spotify/current" ||
		r.URL.Path == "/api/modules/spotify/search" || r.URL.Path == "/api/modules/rulo/lab/simulate" ||
		r.URL.Path == "/api/modules/rulo/lab/compare-rule" {
		client = &http.Client{Timeout: 15 * time.Second}
	} else if strings.HasPrefix(r.URL.Path, "/api/modules/welcome/assets/") {
		client = &http.Client{Timeout: 2 * time.Minute}
	} else if isModuleEventStream(r.URL.Path) {
		client = &http.Client{}
	}
	response, err := client.Do(request)
	if err != nil {
		http.Error(w, "El servidor interno está reiniciando; probá de nuevo en unos segundos", http.StatusServiceUnavailable)
		return
	}
	defer response.Body.Close()
	for _, header := range []string{"Content-Type", "Content-Disposition", "Content-Length", "Content-Range", "Accept-Ranges", "Cache-Control", "ETag", "Last-Modified", "Location"} {
		if value := response.Header.Get(header); value != "" {
			w.Header().Set(header, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	if isModuleEventStream(r.URL.Path) {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
			_ = copyEventStream(w, response.Body, flusher)
			return
		}
		http.Error(w, "La conexión no admite eventos en tiempo real", http.StatusInternalServerError)
		return
	}
	_, _ = io.Copy(w, response.Body)
}

func isModuleEventStream(path string) bool {
	return path == "/api/modules/lights/events" || path == "/api/modules/weather/events" ||
		path == "/api/modules/facebook/events" || path == "/api/modules/facebook/featured/events"
}

func copyEventStream(w http.ResponseWriter, source io.Reader, flusher http.Flusher) error {
	buffer := make([]byte, 4096)
	for {
		read, readErr := source.Read(buffer)
		if read > 0 {
			if _, writeErr := w.Write(buffer[:read]); writeErr != nil {
				return writeErr
			}
			flusher.Flush()
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
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
