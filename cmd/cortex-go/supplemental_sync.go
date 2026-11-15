package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	supplementalSyncTargetFile = "data/supplemental-sync-target.txt"
	maxSupplementalFileBytes   = 8 << 20
	maxSupplementalTotalBytes  = 32 << 20
)

var supplementalSyncPaths = []string{
	"Desplegar Cortex Go.bat",
	"deploy-cortex-go.ps1",
	"cmd/cortex-go/web/rulo/lab.html",
	"cmd/cortex-go/web/rulo/presentation.html",
	"cmd/cortex-go/web/rulo/unified-overlay.html",
	"cmd/cortex-go/web/spotify/dashboard.html",
	"cmd/cortex-go/web/weather/images/flags/ar.svg",
	"cmd/cortex-go/web/weather/images/flags/au.svg",
	"docs/23-MODULO-SPOTIFY.md",
}

func isSupplementalSyncPath(candidate string) bool {
	candidate = filepath.ToSlash(filepath.Clean(filepath.FromSlash(candidate)))
	for _, allowed := range supplementalSyncPaths {
		if strings.EqualFold(candidate, allowed) {
			return true
		}
	}
	return false
}

type supplementalSyncManager struct {
	root   string
	mu     sync.Mutex
	client *http.Client
}

type supplementalSyncFileResult struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

type supplementalSyncResult struct {
	State     string                       `json:"state"`
	Uploaded  int                          `json:"uploaded"`
	Identical int                          `json:"identical"`
	Conflicts int                          `json:"conflicts"`
	Failed    int                          `json:"failed"`
	Files     []supplementalSyncFileResult `json:"files"`
	Message   string                       `json:"message"`
}

func newSupplementalSyncManager(root string) *supplementalSyncManager {
	if root != "" {
		root = filepath.Clean(root)
	}
	return &supplementalSyncManager{
		root: root,
		client: &http.Client{
			Timeout: 3 * time.Minute,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (m *supplementalSyncManager) Sync(ctx context.Context) (supplementalSyncResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := supplementalSyncResult{State: "synced", Files: make([]supplementalSyncFileResult, 0, len(supplementalSyncPaths))}
	if m == nil || m.root == "" {
		return result, errors.New("no se encontró la carpeta local del proyecto Cortex Go")
	}

	base, err := m.loadTarget()
	if err != nil {
		return result, err
	}
	exists, err := m.webDAVExists(ctx, base)
	if err != nil {
		return result, err
	}
	if !exists {
		return result, errors.New("la carpeta WebDAV configurada no existe o no está disponible")
	}

	var totalBytes int64
	for _, relativePath := range supplementalSyncPaths {
		item := supplementalSyncFileResult{Path: relativePath}
		fullPath, pathErr := safeRepositoryPath(m.root, relativePath)
		if pathErr != nil {
			item.State = "failed"
			item.Detail = "La ruta local no es segura."
			result.Failed++
			result.Files = append(result.Files, item)
			continue
		}
		info, statErr := os.Lstat(fullPath)
		if errors.Is(statErr, os.ErrNotExist) {
			item.State = "missing_local"
			item.Detail = "Este archivo no existe en esta PC."
			result.Files = append(result.Files, item)
			continue
		}
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			item.State = "failed"
			item.Detail = "No se pudo leer como archivo normal."
			result.Failed++
			result.Files = append(result.Files, item)
			continue
		}
		if info.Size() > maxSupplementalFileBytes || totalBytes+info.Size() > maxSupplementalTotalBytes {
			item.State = "failed"
			item.Detail = "Supera el límite de tamaño de sincronización."
			result.Failed++
			result.Files = append(result.Files, item)
			continue
		}
		content, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			item.State = "failed"
			item.Detail = "No se pudo leer el archivo local."
			result.Failed++
			result.Files = append(result.Files, item)
			continue
		}
		totalBytes += int64(len(content))

		target, targetErr := supplementalTargetURL(base, relativePath)
		if targetErr != nil {
			item.State = "failed"
			item.Detail = "No se pudo formar la ruta remota."
			result.Failed++
			result.Files = append(result.Files, item)
			continue
		}
		remoteExists, remoteErr := m.webDAVExists(ctx, target)
		if remoteErr != nil {
			item.State = "failed"
			item.Detail = "No se pudo consultar el archivo en WebDAV."
			result.Failed++
			result.Files = append(result.Files, item)
			continue
		}
		if remoteExists {
			remoteContent, readRemoteErr := m.webDAVRead(ctx, target)
			if readRemoteErr != nil {
				item.State = "conflict"
				item.Detail = "El destino ya contiene esta ruta y no se pudo comparar; se conservó sin sobrescribir."
				result.Conflicts++
				result.Files = append(result.Files, item)
				continue
			}
			if bytes.Equal(content, remoteContent) {
				item.State = "identical"
				item.Detail = "Ya estaba sincronizado."
				result.Identical++
			} else {
				item.State = "conflict"
				item.Detail = "Existe una versión diferente en el destino; no se sobrescribió."
				result.Conflicts++
			}
			result.Files = append(result.Files, item)
			continue
		}

		if dirErr := m.ensureWebDAVParents(ctx, base, relativePath); dirErr != nil {
			item.State = "failed"
			item.Detail = "No se pudieron preparar las carpetas remotas."
			result.Failed++
			result.Files = append(result.Files, item)
			continue
		}
		putErr := m.webDAVPut(ctx, target, relativePath, content)
		if errors.Is(putErr, errWebDAVTargetAppeared) {
			item.State = "conflict"
			item.Detail = "El destino cambió durante la operación; el archivo se conservó."
			result.Conflicts++
		} else if putErr != nil {
			item.State = "failed"
			item.Detail = "No se pudo subir el archivo al destino WebDAV."
			result.Failed++
		} else {
			item.State = "uploaded"
			item.Detail = "Enviado al destino WebDAV."
			result.Uploaded++
		}
		result.Files = append(result.Files, item)
	}

	switch {
	case result.Failed > 0 && (result.Uploaded > 0 || result.Identical > 0 || result.Conflicts > 0):
		result.State = "partial"
	case result.Failed > 0:
		result.State = "failed"
	case result.Conflicts > 0 && result.Uploaded > 0:
		result.State = "partial"
	case result.Conflicts > 0:
		result.State = "conflicts"
	case result.Uploaded > 0:
		result.State = "synced"
	default:
		result.State = "up_to_date"
	}
	result.Message = fmt.Sprintf("%d archivo(s) enviados, %d ya estaban iguales y %d se conservaron porque el destino tiene otra versión.", result.Uploaded, result.Identical, result.Conflicts)
	if result.Failed > 0 {
		result.Message += fmt.Sprintf(" No se pudieron procesar %d archivo(s).", result.Failed)
	}
	return result, nil
}

var errWebDAVTargetAppeared = errors.New("el archivo remoto apareció durante la carga")

func (m *supplementalSyncManager) loadTarget() (*url.URL, error) {
	if m.root == "" {
		return nil, errors.New("no se encontró la carpeta local del proyecto Cortex Go")
	}
	configPath := filepath.Join(m.root, filepath.FromSlash(supplementalSyncTargetFile))
	info, err := os.Stat(configPath)
	if err != nil {
		return nil, errors.New("configurá el destino WebDAV en data/supplemental-sync-target.txt")
	}
	if info.Size() > 4096 {
		return nil, errors.New("el archivo de destino WebDAV supera el límite permitido")
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, errors.New("no se pudo leer la configuración local del destino WebDAV")
	}
	target, err := url.Parse(strings.TrimSpace(string(content)))
	if err != nil || !target.IsAbs() || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		return nil, errors.New("el destino WebDAV debe ser una URL http(s) local, sin usuario, contraseña, consulta ni fragmento")
	}
	if !isTailnetWebDAVHost(target.Hostname()) {
		return nil, errors.New("el destino WebDAV debe pertenecer a una dirección Tailscale (.ts.net o 100.64.0.0/10)")
	}
	for _, segment := range strings.Split(target.Path, "/") {
		if segment == "." || segment == ".." {
			return nil, errors.New("la ruta del destino WebDAV contiene un segmento no permitido")
		}
	}
	target.Path = strings.TrimRight(target.Path, "/")
	target.RawPath = ""
	return target, nil
}

func isTailnetWebDAVHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if strings.HasSuffix(host, ".ts.net") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return false
	}
	ipv4 := ip.To4()
	return ipv4[0] == 100 && ipv4[1] >= 64 && ipv4[1] <= 127
}

func supplementalTargetURL(base *url.URL, relativePath string) (*url.URL, error) {
	segments := strings.Split(filepath.ToSlash(relativePath), "/")
	joined, err := url.JoinPath(base.String(), segments...)
	if err != nil {
		return nil, err
	}
	return url.Parse(joined)
}

func (m *supplementalSyncManager) webDAVExists(ctx context.Context, target *url.URL) (bool, error) {
	body := strings.NewReader(`<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/></d:prop></d:propfind>`)
	request, err := http.NewRequestWithContext(ctx, "PROPFIND", target.String(), body)
	if err != nil {
		return false, errors.New("no se pudo preparar la consulta WebDAV")
	}
	request.Header.Set("Depth", "0")
	request.Header.Set("Content-Type", "application/xml; charset=utf-8")
	response, err := m.client.Do(request)
	if err != nil {
		return false, errors.New("no se pudo conectar con el destino WebDAV")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode != http.StatusMultiStatus && (response.StatusCode < 200 || response.StatusCode >= 300) {
		return false, fmt.Errorf("WebDAV respondió HTTP %d al consultar una ruta", response.StatusCode)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	return true, nil
}

func (m *supplementalSyncManager) webDAVRead(ctx context.Context, target *url.URL) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, errors.New("no se pudo preparar la lectura WebDAV")
	}
	response, err := m.client.Do(request)
	if err != nil {
		return nil, errors.New("no se pudo leer el archivo remoto")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("WebDAV respondió HTTP %d al leer un archivo", response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maxSupplementalFileBytes+1))
	if err != nil || len(content) > maxSupplementalFileBytes {
		return nil, errors.New("no se pudo leer el archivo remoto dentro del límite permitido")
	}
	return content, nil
}

func (m *supplementalSyncManager) ensureWebDAVParents(ctx context.Context, base *url.URL, relativePath string) error {
	segments := strings.Split(filepath.ToSlash(relativePath), "/")
	if len(segments) < 2 {
		return nil
	}
	current := *base
	for _, segment := range segments[:len(segments)-1] {
		joined, err := url.JoinPath(current.String(), segment)
		if err != nil {
			return errors.New("no se pudo formar una carpeta WebDAV")
		}
		parsed, err := url.Parse(joined)
		if err != nil {
			return errors.New("no se pudo formar una carpeta WebDAV")
		}
		current = *parsed
		exists, err := m.webDAVExists(ctx, &current)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		request, err := http.NewRequestWithContext(ctx, "MKCOL", current.String(), nil)
		if err != nil {
			return errors.New("no se pudo preparar una carpeta WebDAV")
		}
		response, err := m.client.Do(request)
		if err != nil {
			return errors.New("no se pudo crear una carpeta WebDAV")
		}
		_ = response.Body.Close()
		if response.StatusCode == http.StatusCreated || response.StatusCode == http.StatusOK || response.StatusCode == http.StatusNoContent {
			continue
		}
		if response.StatusCode == http.StatusMethodNotAllowed {
			exists, err = m.webDAVExists(ctx, &current)
			if err == nil && exists {
				continue
			}
		}
		return fmt.Errorf("WebDAV respondió HTTP %d al crear una carpeta", response.StatusCode)
	}
	return nil
}

func (m *supplementalSyncManager) webDAVPut(ctx context.Context, target *url.URL, relativePath string, content []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), bytes.NewReader(content))
	if err != nil {
		return errors.New("no se pudo preparar la carga WebDAV")
	}
	request.Header.Set("Content-Type", mime.TypeByExtension(filepath.Ext(relativePath)))
	request.Header.Set("If-None-Match", "*")
	response, err := m.client.Do(request)
	if err != nil {
		return errors.New("no se pudo enviar el archivo por WebDAV")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusPreconditionFailed {
		return errWebDAVTargetAppeared
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("WebDAV respondió HTTP %d al enviar un archivo", response.StatusCode)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	return nil
}
