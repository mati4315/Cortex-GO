package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const publicGitHubRepository = "mati4315/Cortex-GO"

const maxGitPreviewBytes = 1 << 20
const maxGitBinaryAssetBytes = 20 << 20

type gitSyncManager struct {
	root string
	git  string
	mu   sync.Mutex
}

type gitChange struct {
	Path        string `json:"path"`
	Status      string `json:"status"`
	CanPublish  bool   `json:"canPublish"`
	Review      string `json:"review,omitempty"`
	PreviewDiff string `json:"previewDiff,omitempty"`
}

type gitPreview struct {
	Repository   string      `json:"repository"`
	Branch       string      `json:"branch"`
	Upstream     string      `json:"upstream"`
	Changes      []gitChange `json:"changes"`
	Snapshot     string      `json:"snapshot"`
	CanPublish   bool        `json:"canPublish"`
	Message      string      `json:"message"`
	HasStaged    bool        `json:"hasStaged"`
	Ahead        int         `json:"ahead"`
	Behind       int         `json:"behind"`
	PreviewBytes int         `json:"previewBytes"`
}

type gitPublishRequest struct {
	Message       string   `json:"message"`
	Snapshot      string   `json:"snapshot"`
	Files         []string `json:"files"`
	ConfirmPublic bool     `json:"confirmPublic"`
}

type gitPublishResult struct {
	State      string `json:"state"`
	Repository string `json:"repository"`
	Commit     string `json:"commit,omitempty"`
	Message    string `json:"message"`
	FileCount  int    `json:"fileCount,omitempty"`
}

type gitStatusEntry struct {
	Path   string
	Status string
	Staged bool
}

type gitStatusSnapshot struct {
	Preview gitPreview
	Head    string
	Files   []gitStatusEntry
}

func newGitSyncManager(root, gitPath string) *gitSyncManager {
	if root != "" {
		root = filepath.Clean(root)
	}
	return &gitSyncManager{root: root, git: gitPath}
}

func discoverProjectRoot() string {
	starts := make([]string, 0, 2)
	if executable, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(executable))
	}
	if workingDirectory, err := os.Getwd(); err == nil {
		starts = append(starts, workingDirectory)
	}
	for _, start := range starts {
		current, err := filepath.Abs(start)
		if err != nil {
			continue
		}
		for depth := 0; depth < 8; depth++ {
			if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
				return filepath.Clean(current)
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	return ""
}

func findGitExecutable() string {
	if path, err := exec.LookPath("git"); err == nil {
		return path
	}
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "cmd", "git.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "cmd", "git.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Git", "cmd", "git.exe"),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func (g *gitSyncManager) Preview(ctx context.Context) (gitPreview, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	snapshot, err := g.loadSnapshot(ctx)
	if err != nil {
		return gitPreview{}, err
	}
	return snapshot.Preview, nil
}

func (g *gitSyncManager) Publish(ctx context.Context, request gitPublishRequest) (gitPublishResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	message := strings.TrimSpace(request.Message)
	if message == "" || len([]rune(message)) > 120 || strings.ContainsAny(message, "\r\n\x00") {
		return gitPublishResult{}, errors.New("escribí un mensaje de commit de una sola línea, de hasta 120 caracteres")
	}
	if !request.ConfirmPublic {
		return gitPublishResult{}, errors.New("confirmá que entendés que el código se publicará en GitHub")
	}

	snapshot, err := g.loadSnapshot(ctx)
	if err != nil {
		return gitPublishResult{}, err
	}
	preview := snapshot.Preview
	if !preview.CanPublish {
		return gitPublishResult{}, errors.New(preview.Message)
	}
	if request.Snapshot == "" || request.Snapshot != preview.Snapshot {
		return gitPublishResult{}, errors.New("los archivos cambiaron desde la revisión; volvé a revisar la lista")
	}
	requestedFiles := append([]string(nil), request.Files...)
	sort.Strings(requestedFiles)
	actualFiles := make([]string, 0, len(snapshot.Files))
	for _, entry := range snapshot.Files {
		actualFiles = append(actualFiles, entry.Path)
	}
	sort.Strings(actualFiles)
	if !sameStrings(requestedFiles, actualFiles) {
		return gitPublishResult{}, errors.New("la lista de archivos a publicar cambió; volvé a revisarla")
	}

	if _, err := g.run(ctx, "diff", "--check"); err != nil {
		return gitPublishResult{}, fmt.Errorf("corregí los espacios señalados antes de publicar: %w", err)
	}
	arguments := []string{"add", "--"}
	for _, entry := range snapshot.Files {
		arguments = append(arguments, entry.Path)
	}
	if _, err := g.run(ctx, arguments...); err != nil {
		return gitPublishResult{}, fmt.Errorf("no se pudieron preparar solo los archivos revisados: %w", err)
	}

	stagedOutput, err := g.run(ctx, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return gitPublishResult{}, fmt.Errorf("no se pudo verificar la lista preparada: %w", err)
	}
	stagedFiles := splitGitPaths(stagedOutput)
	sort.Strings(stagedFiles)
	if !sameStrings(stagedFiles, actualFiles) {
		return gitPublishResult{}, errors.New("Git preparó archivos distintos a los revisados; no se creó el commit")
	}
	if _, err := g.run(ctx, "commit", "-m", message); err != nil {
		return gitPublishResult{}, fmt.Errorf("no se pudo crear el commit: %w", err)
	}
	commit, err := g.run(ctx, "rev-parse", "--short", "HEAD")
	if err != nil {
		return gitPublishResult{}, fmt.Errorf("el commit se creó, pero no se pudo leer su identificador: %w", err)
	}
	commit = strings.TrimSpace(commit)
	if _, err := g.run(ctx, "push", "origin", "HEAD:main"); err != nil {
		return gitPublishResult{
			State:      "committed_not_pushed",
			Repository: publicGitHubRepository,
			Commit:     commit,
			FileCount:  len(actualFiles),
			Message:    "El commit quedó guardado localmente, pero GitHub no lo recibió. Revisá la conexión o la autenticación y publicalo luego desde Git.",
		}, err
	}
	return gitPublishResult{
		State:      "published",
		Repository: publicGitHubRepository,
		Commit:     commit,
		FileCount:  len(actualFiles),
		Message:    "Los archivos revisados se publicaron en el repositorio público.",
	}, nil
}

func (g *gitSyncManager) loadSnapshot(ctx context.Context) (gitStatusSnapshot, error) {
	if g == nil || g.root == "" {
		return gitStatusSnapshot{}, errors.New("no se encontró la carpeta local del repositorio Cortex Go")
	}
	if g.git == "" {
		return gitStatusSnapshot{}, errors.New("no se encontró Git en esta computadora")
	}

	branch, err := g.run(ctx, "branch", "--show-current")
	if err != nil {
		return gitStatusSnapshot{}, fmt.Errorf("no se pudo consultar la rama actual: %w", err)
	}
	branch = strings.TrimSpace(branch)
	remote, err := g.run(ctx, "remote", "get-url", "origin")
	if err != nil {
		return gitStatusSnapshot{}, fmt.Errorf("no se encontró el remoto origin: %w", err)
	}
	if !isExpectedGitHubRemote(strings.TrimSpace(remote)) {
		return gitStatusSnapshot{}, fmt.Errorf("el remoto origin no coincide con el repositorio público esperado %s", publicGitHubRepository)
	}
	head, err := g.run(ctx, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return gitStatusSnapshot{}, errors.New("la rama todavía no tiene un commit inicial")
	}
	head = strings.TrimSpace(head)
	upstream, err := g.run(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return gitStatusSnapshot{}, errors.New("main no tiene una rama remota de seguimiento configurada")
	}
	upstream = strings.TrimSpace(upstream)
	if branch != "main" || upstream != "origin/main" {
		return gitStatusSnapshot{}, fmt.Errorf("el atajo publica únicamente la rama main de %s; ahora está en %s (%s)", publicGitHubRepository, branch, upstream)
	}

	aheadText, err := g.run(ctx, "rev-list", "--count", upstream+"..HEAD")
	if err != nil {
		return gitStatusSnapshot{}, fmt.Errorf("no se pudo consultar el historial local: %w", err)
	}
	behindText, err := g.run(ctx, "rev-list", "--count", "HEAD.."+upstream)
	if err != nil {
		return gitStatusSnapshot{}, fmt.Errorf("no se pudo consultar el historial remoto: %w", err)
	}
	ahead := parseGitCount(aheadText)
	behind := parseGitCount(behindText)
	statusOutput, err := g.run(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return gitStatusSnapshot{}, fmt.Errorf("no se pudo revisar el árbol de trabajo: %w", err)
	}
	entries, hasStaged, err := parseGitStatus(statusOutput)
	if err != nil {
		return gitStatusSnapshot{}, err
	}

	preview := gitPreview{
		Repository: publicGitHubRepository,
		Branch:     branch,
		Upstream:   upstream,
		Changes:    make([]gitChange, 0, len(entries)),
		HasStaged:  hasStaged,
		Ahead:      ahead,
		Behind:     behind,
	}
	snapshotHash := sha256.New()
	_, _ = fmt.Fprintf(snapshotHash, "%s\x00%s\x00%s\x00", head, branch, upstream)
	snapshot := gitStatusSnapshot{Head: head}
	previewSize := 0
	allChangesAllowed := true
	for _, entry := range entries {
		change := gitChange{Path: entry.Path, Status: entry.Status}
		if !publishablePath(entry.Path) {
			change.Review = "Este tipo de archivo no se publica desde Cortex Go."
			allChangesAllowed = false
		} else if entry.Staged {
			change.Review = "Hay cambios preparados en el índice de Git; deshacelos o publicalos manualmente antes de usar este atajo."
			allChangesAllowed = false
		} else {
			content, contentErr := g.readWorkingFile(ctx, entry)
			if contentErr != nil {
				change.Review = contentErr.Error()
				allChangesAllowed = false
			} else {
				_, _ = fmt.Fprintf(snapshotHash, "%s\x00%s\x00", entry.Status, entry.Path)
				_, _ = snapshotHash.Write(content)
				_, _ = snapshotHash.Write([]byte{0})
				diff, diffErr := g.fileDiff(ctx, entry, content)
				if diffErr != nil {
					change.Review = diffErr.Error()
					allChangesAllowed = false
				} else {
					previewSize += len(diff)
					if previewSize > maxGitPreviewBytes {
						change.Review = "La vista previa supera 1 MiB; revisá y publicá estos cambios manualmente."
						allChangesAllowed = false
					} else {
						change.CanPublish = true
						change.PreviewDiff = diff
						snapshot.Files = append(snapshot.Files, entry)
					}
				}
			}
		}
		preview.Changes = append(preview.Changes, change)
	}
	preview.PreviewBytes = previewSize
	preview.Snapshot = hex.EncodeToString(snapshotHash.Sum(nil))

	switch {
	case branch != "main":
		preview.Message = "El atajo solo publica desde main."
	case hasStaged:
		preview.Message = "Hay archivos preparados en Git. Revisalos o deshacelos antes de publicar desde el panel."
	case ahead > 0:
		preview.Message = "Hay commits locales anteriores pendientes de Push. Publicalos manualmente para revisar su contenido."
	case behind > 0:
		preview.Message = "GitHub tiene commits que esta carpeta todavía no descargó. Hacé Pull antes de publicar."
	case len(entries) == 0:
		preview.Message = "No hay cambios locales para publicar."
	case !allChangesAllowed:
		preview.Message = "Hay archivos que este atajo no publicará. Revisá las marcas de la lista y usá Git manualmente si corresponde."
	default:
		preview.CanPublish = true
		preview.Message = "Revisá los archivos y las diferencias. Si los confirmás, se crearán un commit y un Push público."
	}
	snapshot.Preview = preview
	return snapshot, nil
}

func (g *gitSyncManager) readWorkingFile(ctx context.Context, entry gitStatusEntry) ([]byte, error) {
	fullPath, err := safeRepositoryPath(g.root, entry.Path)
	if err != nil {
		return nil, err
	}
	binaryAsset := publishableBinaryAsset(entry.Path)
	info, err := os.Lstat(fullPath)
	if errors.Is(err, os.ErrNotExist) && strings.Contains(entry.Status, "D") {
		content, showErr := g.run(ctx, "show", "HEAD:"+entry.Path)
		if showErr != nil {
			return nil, errors.New("no se pudo leer el archivo eliminado para revisarlo")
		}
		deletedContent := []byte(content)
		maxSize := maxGitPreviewBytes
		if binaryAsset {
			maxSize = maxGitBinaryAssetBytes
		}
		if len(deletedContent) > maxSize || (!binaryAsset && (!utf8.Valid(deletedContent) || bytes.IndexByte(deletedContent, 0) >= 0)) {
			return nil, errors.New("el archivo eliminado supera el límite o no se puede revisar de forma segura")
		}
		return deletedContent, nil
	}
	if err != nil {
		return nil, errors.New("no se pudo abrir uno de los archivos modificados")
	}
	if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
		return nil, errors.New("no se publican vínculos ni carpetas desde este atajo")
	}
	maxSize := int64(maxGitPreviewBytes)
	if binaryAsset {
		maxSize = maxGitBinaryAssetBytes
	}
	if info.Size() > maxSize {
		return nil, errors.New("el archivo supera el límite permitido para revisión")
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, errors.New("no se pudo leer uno de los archivos modificados")
	}
	if !binaryAsset && (!utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0) {
		return nil, errors.New("no se publican archivos binarios desde este atajo")
	}
	return content, nil
}

func (g *gitSyncManager) fileDiff(ctx context.Context, entry gitStatusEntry, content []byte) (string, error) {
	if publishableBinaryAsset(entry.Path) {
		digest := sha256.Sum256(content)
		return fmt.Sprintf("Binary asset review\nPath: %s\nStatus: %s\nSize: %d bytes\nSHA-256: %x\n", entry.Path, entry.Status, len(content), digest), nil
	}
	if strings.Contains(entry.Status, "D") {
		oldLines := strings.SplitAfter(string(content), "\n")
		var diff strings.Builder
		fmt.Fprintf(&diff, "--- a/%s\n+++ /dev/null\n", entry.Path)
		for _, line := range oldLines {
			if line == "" {
				continue
			}
			diff.WriteByte('-')
			diff.WriteString(line)
			if !strings.HasSuffix(line, "\n") {
				diff.WriteByte('\n')
			}
		}
		return diff.String(), nil
	}
	if entry.Status == "??" {
		var diff strings.Builder
		fmt.Fprintf(&diff, "--- /dev/null\n+++ b/%s\n", entry.Path)
		for _, line := range strings.SplitAfter(string(content), "\n") {
			if line == "" {
				continue
			}
			diff.WriteByte('+')
			diff.WriteString(line)
			if !strings.HasSuffix(line, "\n") {
				diff.WriteByte('\n')
			}
		}
		return diff.String(), nil
	}
	return g.run(ctx, "diff", "--no-ext-diff", "--no-color", "--unified=3", "--", entry.Path)
}

func (g *gitSyncManager) run(ctx context.Context, args ...string) (string, error) {
	gitArguments := make([]string, 0, len(args)+2)
	gitArguments = append(gitArguments, "-c", "safe.directory="+g.root)
	gitArguments = append(gitArguments, args...)
	command := exec.CommandContext(ctx, g.git, gitArguments...)
	command.Dir = g.root
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 1000 {
			message = message[:1000]
		}
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("%s", message)
	}
	return string(output), nil
}

func parseGitStatus(output string) ([]gitStatusEntry, bool, error) {
	parts := strings.Split(output, "\x00")
	entries := make([]gitStatusEntry, 0, len(parts))
	hasStaged := false
	for index := 0; index < len(parts); index++ {
		item := parts[index]
		if item == "" {
			continue
		}
		if len(item) < 4 || item[2] != ' ' {
			return nil, false, errors.New("Git devolvió un estado de archivos que el atajo no puede revisar")
		}
		status := item[:2]
		path := item[3:]
		if strings.ContainsAny(status, "RCU") {
			return nil, false, errors.New("hay archivos renombrados o conflictos; resolvelos con Git antes de publicar")
		}
		if status[0] != ' ' && status[0] != '?' {
			hasStaged = true
		}
		if status == "??" || status[0] != ' ' || status[1] != ' ' {
			entries = append(entries, gitStatusEntry{
				Path:   path,
				Status: strings.TrimSpace(status),
				Staged: status[0] != ' ' && status[0] != '?',
			})
		}
		if status[0] == 'R' || status[0] == 'C' || status[1] == 'R' || status[1] == 'C' {
			index++ // -z emits the original path as a second entry for renames/copies.
		}
	}
	return entries, hasStaged, nil
}

func publishablePath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "README.md" || clean == "go.mod" || clean == "go.sum" {
		return true
	}
	if strings.HasPrefix(clean, "cmd/cortex-go/") {
		switch clean {
		case "cmd/cortex-go/web/index.html",
			"cmd/cortex-go/web/sw.js",
			"cmd/cortex-go/web/manifest.webmanifest",
			"cmd/cortex-go/web/icons/icon-192.png",
			"cmd/cortex-go/web/icons/icon-512.png":
			return true
		}
		if strings.HasPrefix(clean, "cmd/cortex-go/web/welcome/") {
			if publishableBinaryAsset(clean) {
				return true
			}
			if strings.HasPrefix(clean, "cmd/cortex-go/web/welcome/fonts/") && strings.EqualFold(filepath.Ext(clean), ".txt") {
				return true
			}
			switch strings.ToLower(filepath.Ext(clean)) {
			case ".html", ".css", ".js":
				return true
			default:
				return false
			}
		}
		if strings.HasPrefix(clean, "cmd/cortex-go/web/lights/") {
			switch strings.ToLower(filepath.Ext(clean)) {
			case ".html", ".css", ".js":
				return true
			default:
				return false
			}
		}
		if strings.HasPrefix(clean, "cmd/cortex-go/") {
			extension := strings.ToLower(filepath.Ext(clean))
			return extension == ".go"
		}
	}
	if strings.HasPrefix(clean, "experiments/facebook-browser-poc/") {
		name := filepath.Base(clean)
		extension := strings.ToLower(filepath.Ext(name))
		return name == "README.md" || extension == ".go" || extension == ".mod" || extension == ".sum"
	}
	return strings.HasPrefix(clean, "docs/") && strings.EqualFold(filepath.Ext(clean), ".md")
}

func publishableBinaryAsset(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if strings.HasPrefix(clean, "cmd/cortex-go/web/welcome/images/") {
		return strings.EqualFold(filepath.Ext(clean), ".png")
	}
	if strings.HasPrefix(clean, "cmd/cortex-go/web/welcome/fonts/") {
		return strings.EqualFold(filepath.Ext(clean), ".woff2")
	}
	return false
}

func safeRepositoryPath(root, path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", errors.New("la ruta del cambio no es relativa al repositorio")
	}
	fullPath := filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
	relative, err := filepath.Rel(root, fullPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("la ruta del cambio sale del repositorio")
	}
	return fullPath, nil
}

func isExpectedGitHubRemote(remote string) bool {
	remote = strings.TrimSpace(remote)
	var host, repository string
	if strings.HasPrefix(remote, "git@github.com:") {
		host = "github.com"
		repository = strings.TrimPrefix(remote, "git@github.com:")
	} else {
		parsed, err := url.Parse(remote)
		if err != nil || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "ssh") {
			return false
		}
		host = parsed.Hostname()
		repository = strings.TrimPrefix(parsed.Path, "/")
		if parsed.Scheme == "ssh" && parsed.User != nil && parsed.User.Username() != "git" {
			return false
		}
	}
	repository = strings.TrimSuffix(repository, ".git")
	return strings.EqualFold(host, "github.com") && strings.EqualFold(repository, publicGitHubRepository)
}

func parseGitCount(value string) int {
	var count int
	_, _ = fmt.Sscanf(strings.TrimSpace(value), "%d", &count)
	return count
}

func splitGitPaths(value string) []string {
	paths := strings.Split(value, "\x00")
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != "" {
			result = append(result, filepath.ToSlash(path))
		}
	}
	return result
}

func sameStrings(first, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for i := range first {
		if first[i] != second[i] {
			return false
		}
	}
	return true
}
