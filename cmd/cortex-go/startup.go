package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const startupShortcutName = "Cortex Go.lnk"

const startupShortcutScript = `$ErrorActionPreference = 'Stop'
$startup = [Environment]::GetFolderPath('Startup')
if ([string]::IsNullOrWhiteSpace($startup)) { throw 'Windows no devolvió la carpeta Inicio del usuario.' }
$shortcutPath = Join-Path $startup 'Cortex Go.lnk'
$target = $env:CORTEX_STARTUP_TARGET
$action = $env:CORTEX_STARTUP_ACTION
if ($action -eq 'enable') {
  if ([string]::IsNullOrWhiteSpace($target) -or -not (Test-Path -LiteralPath $target -PathType Leaf)) { throw 'No se encontró el ejecutable de Cortex Go.' }
  $shell = New-Object -ComObject WScript.Shell
  $shortcut = $shell.CreateShortcut($shortcutPath)
  $shortcut.TargetPath = $target
  $shortcut.Arguments = '--no-browser'
  $shortcut.WorkingDirectory = $env:CORTEX_STARTUP_WORKDIR
  $shortcut.Description = 'Inicia Cortex Go al iniciar sesión en Windows'
  $shortcut.IconLocation = "$target,0"
  $shortcut.WindowStyle = 7
  $shortcut.Save()
  [Console]::Out.Write('enabled')
} elseif ($action -eq 'disable') {
  if (Test-Path -LiteralPath $shortcutPath) {
    $item = Get-Item -LiteralPath $shortcutPath -Force
    if ($item.PSIsContainer) { throw 'La ruta del acceso directo es una carpeta y no se modificó.' }
    Remove-Item -LiteralPath $shortcutPath -Force
  }
  [Console]::Out.Write('disabled')
} else {
  throw 'La acción del inicio automático no es válida.'
}`

type startupSetting struct {
	Supported bool   `json:"supported"`
	Enabled   bool   `json:"enabled"`
	Message   string `json:"message,omitempty"`
}

func (c *controlServer) handleStartupSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		setting, err := currentStartupSetting()
		if err != nil {
			http.Error(w, "No se pudo leer la configuración de inicio automático", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, setting)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "El ajuste de inicio automático no es válido", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "El ajuste contiene datos adicionales", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	if err := setStartupEnabled(ctx, input.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	setting, err := currentStartupSetting()
	if err != nil {
		http.Error(w, "El cambio se aplicó, pero no se pudo comprobar el estado", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, setting)
}

func currentStartupSetting() (startupSetting, error) {
	if runtime.GOOS != "windows" {
		return startupSetting{Supported: false, Message: "El inicio automático desde el panel solo está disponible en Windows."}, nil
	}
	path, err := startupShortcutPath()
	if err != nil {
		return startupSetting{}, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return startupSetting{Supported: true}, nil
	}
	if err != nil {
		return startupSetting{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return startupSetting{}, errors.New("la ruta reservada para el inicio de Cortex no es un archivo regular")
	}
	return startupSetting{Supported: true, Enabled: true}, nil
}

func startupShortcutPath() (string, error) {
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData == "" {
		return "", errors.New("Windows no definió la carpeta de datos del usuario")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", startupShortcutName), nil
}

func setStartupEnabled(ctx context.Context, enabled bool) error {
	if runtime.GOOS != "windows" {
		return errors.New("el inicio automático desde el panel solo está disponible en Windows")
	}
	path, err := startupShortcutPath()
	if err != nil {
		return err
	}
	info, statErr := os.Lstat(path)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("no se pudo revisar el acceso directo de inicio: %w", statErr)
	}
	if statErr == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return errors.New("la ruta reservada para el inicio de Cortex no es un archivo regular; no se modificó")
	}
	if !enabled {
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("no se pudo desactivar el inicio automático: %w", err)
		}
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo identificar Cortex Go: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("no se pudo resolver la ruta de Cortex Go: %w", err)
	}
	command := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", startupShortcutScript)
	command.Env = startupCommandEnvironment(os.Environ(), map[string]string{
		"CORTEX_STARTUP_ACTION":  "enable",
		"CORTEX_STARTUP_TARGET":  executable,
		"CORTEX_STARTUP_WORKDIR": filepath.Dir(filepath.Dir(executable)),
	})
	configureHiddenProcess(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("no se pudo habilitar el inicio automático: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func startupCommandEnvironment(environment []string, values map[string]string) []string {
	filtered := make([]string, 0, len(environment)+len(values))
	for _, entry := range environment {
		key, _, found := strings.Cut(entry, "=")
		if !found {
			filtered = append(filtered, entry)
			continue
		}
		replaced := false
		for name := range values {
			if strings.EqualFold(key, name) {
				replaced = true
				break
			}
		}
		if !replaced {
			filtered = append(filtered, entry)
		}
	}
	for name, value := range values {
		filtered = append(filtered, name+"="+value)
	}
	return filtered
}
