package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitSyncPreviewLimitsFilesAndRequiresPublicConfirmation(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("Git no está disponible en el entorno de pruebas")
	}
	directory := t.TempDir()
	repository := filepath.Join(directory, "Cortex-GO")
	remote := filepath.Join(directory, "origin.git")
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, directory, "init", "--quiet", "--bare", "--initial-branch=main", remote)
	runTestGit(t, repository, "init", "--quiet", "--initial-branch=main")
	runTestGit(t, repository, "config", "user.name", "Cortex Test")
	runTestGit(t, repository, "config", "user.email", "cortex-test@example.invalid")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("# Cortex Go\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, repository, "add", "README.md")
	runTestGit(t, repository, "commit", "--quiet", "-m", "base")
	runTestGit(t, repository, "remote", "add", "origin", remote)
	runTestGit(t, repository, "push", "--quiet", "--set-upstream", "origin", "main")
	runTestGit(t, repository, "remote", "set-url", "origin", "https://github.com/mati4315/Cortex-GO.git")

	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("# Cortex Go\nCambio para revisar\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repository, "cmd", "cortex-go"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "cmd", "cortex-go", "sample.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := newGitSyncManager(repository, gitPath)
	preview, err := manager.Preview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !preview.CanPublish || preview.Repository != publicGitHubRepository || preview.Branch != "main" {
		t.Fatalf("la vista previa del repo limpio no quedó habilitada: %#v", preview)
	}
	if len(preview.Changes) != 2 {
		t.Fatalf("se esperaban dos archivos revisables, se encontraron %#v", preview.Changes)
	}
	if !strings.Contains(preview.Changes[0].PreviewDiff, "Cambio para revisar") && !strings.Contains(preview.Changes[1].PreviewDiff, "Cambio para revisar") {
		t.Fatalf("la vista previa no muestra el contenido del cambio: %#v", preview.Changes)
	}
	if _, err := manager.Publish(context.Background(), gitPublishRequest{
		Message:  "publicación sin marcar",
		Snapshot: preview.Snapshot,
		Files:    []string{"README.md", "cmd/cortex-go/sample.go"},
	}); err == nil || !strings.Contains(err.Error(), "confirmá que entendés") {
		t.Fatalf("la publicación sin confirmación pública no fue bloqueada: %v", err)
	}
	if _, err := manager.Publish(context.Background(), gitPublishRequest{
		Message:       "revisión obsoleta",
		Snapshot:      "snapshot-obsoleto",
		Files:         []string{"README.md", "cmd/cortex-go/sample.go"},
		ConfirmPublic: true,
	}); err == nil || !strings.Contains(err.Error(), "cambiaron desde la revisión") {
		t.Fatalf("la publicación con una vista previa obsoleta no fue bloqueada: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(repository, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "data", "test.sqlite"), []byte("datos ficticios"), 0o600); err != nil {
		t.Fatal(err)
	}
	blocked, err := manager.Preview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if blocked.CanPublish {
		t.Fatalf("el atajo permitió publicar una base local: %#v", blocked)
	}
	foundBlockedDatabase := false
	for _, change := range blocked.Changes {
		if change.Path == "data/test.sqlite" && !change.CanPublish {
			foundBlockedDatabase = true
		}
	}
	if !foundBlockedDatabase {
		t.Fatalf("la base local no aparece bloqueada en la revisión: %#v", blocked.Changes)
	}
}

func TestPublishablePathExcludesPrivateAndGeneratedFiles(t *testing.T) {
	allowed := []string{
		"README.md", "go.mod", "go.sum", "cmd/cortex-go/store.go",
		"cmd/cortex-go/web/index.html", "cmd/cortex-go/web/sw.js",
		"cmd/cortex-go/web/manifest.webmanifest", "cmd/cortex-go/web/icons/icon-192.png",
		"experiments/facebook-browser-poc/main.go", "experiments/facebook-browser-poc/go.mod",
		"experiments/facebook-browser-poc/README.md", "docs/12-SINCRONIZACION-ENTRE-PC.md",
	}
	for _, path := range allowed {
		if !publishablePath(path) {
			t.Errorf("debería permitirse revisar %q", path)
		}
	}
	blocked := []string{
		".gitignore", ".env", "data/first-cut.sqlite", "data/backups/latest.sqlite", "logs/cortex.log",
		"img programa real/stream.png", "bin/CortexGo.exe", "secret.json",
		"cmd/cortex-go/web/private.png", "cmd/cortex-go/web/icons/other.png",
		"experiments/facebook-browser-poc/.env", "experiments/facebook-browser-poc/private.json",
	}
	for _, path := range blocked {
		if publishablePath(path) {
			t.Errorf("no debería permitirse publicar %q desde el atajo", path)
		}
	}
}

func runTestGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("Git no está disponible en el entorno de pruebas")
	}
	command := exec.Command(gitPath, args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s falló: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
