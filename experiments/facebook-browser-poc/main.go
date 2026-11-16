package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

const (
	bridgeName = "cortexFacebookProbe"
)

var facebookCommentsPathPattern = regexp.MustCompile(`(?i)^/live/producer/dashboard/(\d+)/comments/?$`)

type account struct {
	label    string
	id       string
	feedID   string
	streamID string
	pageURL  string
	port     int
	targetID target.ID
	proc     *exec.Cmd
	cancel   context.CancelFunc
	debugURL string
	saved    int
}

type debuggerVersion struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type debuggerTarget struct {
	ID   target.ID `json:"id"`
	Type string    `json:"type"`
	URL  string    `json:"url"`
}

type domProbe struct {
	Account            string            `json:"account"`
	PageURL            string            `json:"pageUrl"`
	StreamID           string            `json:"streamId"`
	CommentsPage       bool              `json:"commentsPage"`
	VisibleRows        int               `json:"visibleRows"`
	RecognizedComments int               `json:"recognizedComments"`
	NewComments        int               `json:"newComments"`
	LastDetected       int64             `json:"lastDetected"`
	LastError          string            `json:"lastError,omitempty"`
	Comments           []capturedComment `json:"comments"`
}

type capturedComment struct {
	ExternalID    string `json:"externalId"`
	ParticipantID string `json:"participantId"`
	StreamID      string `json:"streamId"`
	Author        string `json:"author"`
	Message       string `json:"message"`
	EventAt       string `json:"eventAt"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Prueba Facebook:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	disableConsoleQuickEdit()
	flags := flag.NewFlagSet("facebook-browser-poc", flag.ContinueOnError)
	accountChoice := flags.Int("account", 0, "cuenta a abrir: 1 o 2; 0 abre todas las URLs configuradas")
	url1 := flags.String("url1", "", "URL del Live Producer para Cuenta 1")
	url2 := flags.String("url2", "", "URL del Live Producer para Cuenta 2")
	cortexURL := flags.String("cortex-url", "http://127.0.0.1:17845", "URL local del panel Cortex Go")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *accountChoice < 0 || *accountChoice > 2 {
		return errors.New("-account debe ser 1, 2 o 0 para abrir todas las URLs configuradas")
	}
	if err := validateCortexURL(*cortexURL); err != nil {
		return err
	}
	if strings.TrimSpace(*url1) == "" && strings.TrimSpace(*url2) == "" {
		var err error
		*url1, *url2, err = promptLiveURLs()
		if err != nil {
			return err
		}
	}
	accounts, err := requestedAccounts(*url1, *url2)
	if err != nil {
		return err
	}
	if *accountChoice != 0 {
		selectedID := fmt.Sprintf("cuenta-%d", *accountChoice)
		selected := make([]*account, 0, 1)
		for _, a := range accounts {
			if a.id == selectedID {
				selected = append(selected, a)
				break
			}
		}
		if len(selected) == 0 {
			return fmt.Errorf("no configuraste una URL para Cuenta %d", *accountChoice)
		}
		accounts = selected
	}
	browserPath, err := findBrave()
	if err != nil {
		return err
	}
	localData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localData == "" {
		return errors.New("Windows no definió LOCALAPPDATA; no se crearán perfiles dentro del repositorio")
	}
	profileRoot := filepath.Join(localData, "CortexGo", "FacebookBrowserPoC", "brave")
	if err := os.MkdirAll(profileRoot, 0o700); err != nil {
		return fmt.Errorf("no se pudo preparar el almacenamiento privado local: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("Captura de Facebook en Brave: %d cuenta(s), un perfil y una pestaña por cada URL.\n", len(accounts))
	fmt.Println("El navegador conserva las cookies en LOCALAPPDATA, fuera del repositorio.")
	fmt.Println("Cada perfil abre una sola pestaña de Live Producer; inicia sesión en la cuenta correcta.")
	fmt.Println("Solo envía comentarios nuevos a Cortex Go; no imprime nombres ni textos en la consola.")
	for _, a := range accounts {
		fmt.Printf("%s: directo %s\n", a.label, a.streamID)
	}
	if len(accounts) == 1 {
		fmt.Printf("Solo se abrirá %s. Cierra con Ctrl+C para cerrar el navegador de forma ordenada.\n", accounts[0].label)
	} else {
		fmt.Println("Cierra con Ctrl+C para cerrar los dos navegadores de forma ordenada.")
	}

	for _, a := range accounts {
		if err := launch(ctx, browserPath, profileRoot, a); err != nil {
			closeAccounts(accounts)
			return err
		}
		if err := connectAndObserve(ctx, a, *cortexURL); err != nil {
			closeAccounts(accounts)
			return fmt.Errorf("no se pudo preparar %s: %w", a.label, err)
		}
	}

	<-ctx.Done()
	closeAccounts(accounts)
	return nil
}

func promptLiveURLs() (string, string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Pega la URL de Comentarios de Live Producer para cada cuenta. Deja una vacía si no la vas a usar.")
	fmt.Print("URL Cuenta 1: ")
	first, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", errors.New("no se pudieron leer las URL de Facebook")
	}
	fmt.Print("URL Cuenta 2: ")
	second, secondErr := reader.ReadString('\n')
	if secondErr != nil && !errors.Is(secondErr, io.EOF) {
		return "", "", errors.New("no se pudieron leer las URL de Facebook")
	}
	return strings.TrimSpace(first), strings.TrimSpace(second), nil
}

func requestedAccounts(firstURL, secondURL string) ([]*account, error) {
	inputs := []struct {
		label, id, feedID, rawURL string
	}{
		{label: "Cuenta 1", id: "cuenta-1", feedID: "a", rawURL: firstURL},
		{label: "Cuenta 2", id: "cuenta-2", feedID: "b", rawURL: secondURL},
	}
	accounts := make([]*account, 0, len(inputs))
	for _, input := range inputs {
		if strings.TrimSpace(input.rawURL) == "" {
			continue
		}
		pageURL, streamID, err := validateFacebookCommentsURL(input.rawURL)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", input.label, err)
		}
		accounts = append(accounts, &account{
			label: input.label, id: input.id, feedID: input.feedID,
			streamID: streamID, pageURL: pageURL,
		})
	}
	if len(accounts) == 0 {
		return nil, errors.New("pega al menos una URL de Comentarios de Live Producer")
	}
	return accounts, nil
}

func validateCortexURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("Cortex debe ser una URL HTTP local sin credenciales ni ruta adicional")
	}
	host := strings.ToLower(u.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return errors.New("por seguridad, el prototipo solo envía eventos a un panel Cortex de loopback")
	}
	return nil
}

func validateFacebookCommentsURL(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 2048 {
		return "", "", errors.New("la URL supera el tamaño permitido")
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.User != nil {
		return "", "", errors.New("usa una URL HTTPS de Facebook Live Producer")
	}
	host := strings.ToLower(u.Hostname())
	if host != "facebook.com" && host != "www.facebook.com" {
		return "", "", errors.New("la URL debe pertenecer a facebook.com")
	}
	match := facebookCommentsPathPattern.FindStringSubmatch(u.Path)
	if len(match) != 2 {
		return "", "", errors.New("usa la ruta /live/producer/dashboard/ID/COMMENTS/")
	}
	streamID := match[1]
	return "https://www.facebook.com/live/producer/dashboard/" + streamID + "/COMMENTS/", streamID, nil
}

func isFacebookCommentsURL(raw, streamID string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host != "facebook.com" && host != "www.facebook.com" {
		return false
	}
	match := facebookCommentsPathPattern.FindStringSubmatch(u.Path)
	return len(match) == 2 && match[1] == streamID
}

func findBrave() (string, error) {
	candidates := []string{
		filepath.Join(os.Getenv("PROGRAMFILES"), "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("no se encontró Brave en las rutas de instalación habituales")
}

func launch(ctx context.Context, browserPath, profileRoot string, a *account) error {
	port, err := freeDebugPort()
	if err != nil {
		return fmt.Errorf("no se pudo reservar un puerto local para %s", a.label)
	}
	a.port = port
	profileDir := filepath.Join(profileRoot, a.id)
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return fmt.Errorf("no se pudo preparar el perfil de %s: %w", a.label, err)
	}

	cmd := exec.Command(browserPath,
		"--user-data-dir="+profileDir,
		"--remote-debugging-address=127.0.0.1",
		"--remote-debugging-port="+strconv.Itoa(a.port),
		"--new-window",
		"--no-first-run",
		"--no-default-browser-check",
		a.pageURL,
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("no se pudo abrir %s: %w", a.label, err)
	}
	a.proc = cmd
	go func() { _ = cmd.Wait() }()
	debugURL, err := waitForDebugger(ctx, a.port, 25*time.Second)
	if err != nil {
		return fmt.Errorf("no se pudo conectar con %s: %w", a.label, err)
	}
	a.debugURL = debugURL
	targetID, err := waitForFacebookTarget(ctx, a.port, 15*time.Second)
	if err != nil {
		return fmt.Errorf("no se encontró la pestaña inicial de Facebook para %s: %w", a.label, err)
	}
	a.targetID = targetID
	return nil
}

func freeDebugPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

func connectAndObserve(parent context.Context, a *account, cortexURL string) error {
	allocator, cancelAllocator := remote.NewAllocator(parent, a.debugURL)
	ctx, cancelTarget := chromedp.NewContext(allocator, chromedp.WithTargetID(a.targetID))
	a.cancel = func() {
		cancelTarget()
		cancelAllocator()
	}

	if err := chromedp.Do(ctx, chromedp.ExposeFunc(bridgeName, func(_ context.Context, payload string) (chromedp.Void, error) {
		var probe domProbe
		if err := json.Unmarshal([]byte(payload), &probe); err != nil || probe.Account != a.id {
			return chromedp.Void{}, nil
		}
		if !isFacebookURL(probe.PageURL) {
			return chromedp.Void{}, nil
		}
		if probe.CommentsPage && (probe.StreamID != a.streamID || !isFacebookCommentsURL(probe.PageURL, a.streamID)) {
			probe.CommentsPage = false
			probe.LastError = "La página abierta no coincide con la URL de esta cuenta."
			probe.Comments = nil
		}
		if err := postCaptureStatus(parent, cortexURL, a, probe); err != nil {
			fmt.Printf("[%s] No se pudo actualizar el estado de captura.\n", a.label)
		}
		captureError := probe.LastError
		for _, comment := range probe.Comments {
			saved, err := postCapturedComment(parent, cortexURL, a, comment)
			if err != nil {
				fmt.Printf("[%s] No se pudo guardar un comentario (error de conexión o respuesta del panel).\n", a.label)
				captureError = "Cortex no pudo guardar un comentario; revisa el estado del servicio."
				continue
			}
			if saved {
				a.saved++
				fmt.Printf("[%s] Comentario nuevo guardado; total de esta sesión: %d.\n", a.label, a.saved)
			} else {
				fmt.Printf("[%s] Comentario repetido descartado por deduplicación.\n", a.label)
			}
		}
		if captureError != probe.LastError {
			probe.LastError = captureError
			if err := postCaptureStatus(parent, cortexURL, a, probe); err != nil {
				fmt.Printf("[%s] No se pudo informar el error de guardado.\n", a.label)
			}
		}
		printProbe(a.label, probe)
		return chromedp.Void{}, nil
	})); err != nil {
		return err
	}
	if _, err := chromedp.Call(ctx, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{
		Source: observerScript(a.id),
	}); err != nil {
		return err
	}
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](observerScript(a.id))); err != nil {
		return err
	}
	return nil
}

func postCapturedComment(parent context.Context, cortexURL string, a *account, comment capturedComment) (bool, error) {
	if comment.StreamID == "" || strings.TrimSpace(comment.Author) == "" || strings.TrimSpace(comment.Message) == "" {
		return false, errors.New("evento incompleto")
	}
	body, err := json.Marshal(map[string]string{
		"feedId": a.feedID, "sourceId": "facebook:" + a.id,
		"externalId": comment.ExternalID, "streamId": comment.StreamID,
		"participantId": comment.ParticipantID,
		"author":        comment.Author, "message": comment.Message, "eventAt": comment.EventAt,
	})
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cortexURL, "/")+"/api/events", strings.NewReader(string(body)))
	if err != nil {
		return false, err
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	response, err := client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	var result struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 16*1024)).Decode(&result); err != nil {
		return false, errors.New("el panel devolvió una respuesta inválida")
	}
	if response.StatusCode == http.StatusCreated && result.State == "saved" {
		return true, nil
	}
	if response.StatusCode == http.StatusOK && result.State == "duplicate" {
		return false, nil
	}
	return false, fmt.Errorf("el panel respondió HTTP %d", response.StatusCode)
}

func postCaptureStatus(parent context.Context, cortexURL string, a *account, probe domProbe) error {
	body, err := json.Marshal(map[string]any{
		"commentsPage":       probe.CommentsPage,
		"visibleRows":        probe.VisibleRows,
		"recognizedComments": probe.RecognizedComments,
		"newComments":        probe.NewComments,
		"lastDetected":       probe.LastDetected,
		"lastError":          probe.LastError,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	endpoint := strings.TrimRight(cortexURL, "/") + "/api/feeds/" + a.feedID + "/status"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 2048))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("el panel respondió HTTP %d", response.StatusCode)
	}
	return nil
}

func waitForDebugger(parent context.Context, port int, timeout time.Duration) (string, error) {
	client := &http.Client{
		Timeout:   600 * time.Millisecond,
		Transport: &http.Transport{Proxy: nil},
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port) + "/json/version"
	for {
		request, err := http.NewRequestWithContext(parent, http.MethodGet, endpoint, nil)
		if err == nil {
			response, requestErr := client.Do(request)
			if requestErr == nil {
				var info debuggerVersion
				decodeErr := json.NewDecoder(io.LimitReader(response.Body, 32*1024)).Decode(&info)
				_ = response.Body.Close()
				if decodeErr == nil && localWebSocketURL(info.WebSocketDebuggerURL) {
					return info.WebSocketDebuggerURL, nil
				}
			}
		}
		select {
		case <-parent.Done():
			return "", parent.Err()
		case <-deadline.C:
			return "", fmt.Errorf("el navegador no habilitó el canal local de control en %s", endpoint)
		case <-ticker.C:
		}
	}
}

func waitForFacebookTarget(parent context.Context, port int, timeout time.Duration) (target.ID, error) {
	client := &http.Client{
		Timeout:   600 * time.Millisecond,
		Transport: &http.Transport{Proxy: nil},
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port) + "/json/list"
	for {
		request, err := http.NewRequestWithContext(parent, http.MethodGet, endpoint, nil)
		if err == nil {
			response, requestErr := client.Do(request)
			if requestErr == nil {
				var targets []debuggerTarget
				decodeErr := json.NewDecoder(io.LimitReader(response.Body, 128*1024)).Decode(&targets)
				_ = response.Body.Close()
				if decodeErr == nil {
					for _, candidate := range targets {
						if candidate.Type == "page" && candidate.ID != "" && isFacebookURL(candidate.URL) {
							return candidate.ID, nil
						}
					}
				}
			}
		}
		select {
		case <-parent.Done():
			return "", parent.Err()
		case <-deadline.C:
			return "", errors.New("Brave no abrió una pestaña de Facebook por DevTools local")
		case <-ticker.C:
		}
	}
}

func observerScript(accountID string) string {
	encoded, _ := json.Marshal(accountID)
	return `(() => {
  const account = ` + string(encoded) + `;
  const windowLabel = account === 'cuenta-1' ? 'Cortex Go — Cuenta 1' : 'Cortex Go — Cuenta 2';
  const marker = '__cortexFacebookProbe_' + account;
  if (window[marker]) return true;
  const state = { route: '', seen: new Set(), baseline: true, newComments: 0, recognizedComments: 0, lastDetected: 0, lastError: '', unrecognizedScans: 0, timer: 0, lastReportAt: 0, lastReportSignature: '' };
  window[marker] = state;
  const normalize = value => String(value || '').replace(/\s+/g, ' ').trim();
  const hash = value => {
    let out = 2166136261;
    for (let i = 0; i < value.length; i++) out = Math.imul(out ^ value.charCodeAt(i), 16777619);
    return (out >>> 0).toString(16);
  };
  const host = location.hostname.toLowerCase();
  const isFacebookHost = host === 'facebook.com' || host.endsWith('.facebook.com');
  const isCommentsPage = () => isFacebookHost && /\/live\/producer\/dashboard\/\d+\/comments(?:\/|$)/i.test(location.pathname);
  function rows() {
    if (!isCommentsPage()) return [];
    return [...document.querySelectorAll('[role="article"], [role="listitem"], [role="row"]')]
      .filter(node => node.getClientRects().length && normalize(node.innerText || node.textContent))
      .slice(-500);
  }
  function keyFor(row, comment) {
    if (comment.externalId) return 'id:' + comment.streamId + ':' + comment.externalId;
    const dataId = row.getAttribute('data-id') || row.id || '';
    if (/^[0-9_]{6,}$/.test(dataId)) return 'id:' + comment.streamId + ':' + dataId;
    return 'text:' + hash([
      comment.streamId,
      comment.participantId || comment.author.toLowerCase(),
      comment.author.toLowerCase(),
      comment.message,
      comment.eventAt || ''
    ].join('\u0000'));
  }
  function report(visibleRows, comments = []) {
    try {
      const bridge = window.` + bridgeName + `;
      if (typeof bridge !== 'function') return;
      const streamId = location.pathname.match(/\/live\/producer\/dashboard\/(\d+)\/comments(?:\/|$)/i)?.[1] || '';
      const signature = [isCommentsPage(), visibleRows, state.recognizedComments, state.newComments, state.lastDetected, state.lastError].join(':');
      const now = Date.now();
      if (signature === state.lastReportSignature && now - state.lastReportAt < 5000) return;
      state.lastReportSignature = signature;
      state.lastReportAt = now;
      bridge(JSON.stringify({
        account,
        pageUrl: isCommentsPage()
          ? 'https://www.facebook.com/live/producer/dashboard/' + streamId + '/COMMENTS/'
          : (isFacebookHost ? location.origin + location.pathname : ''),
        streamId,
        commentsPage: isCommentsPage(),
        visibleRows,
        recognizedComments: state.recognizedComments,
        newComments: state.newComments,
        lastDetected: state.lastDetected,
        lastError: state.lastError,
        comments
      }));
    } catch (_) {}
  }
  function scan() {
    state.timer = 0;
    if (document.title !== windowLabel) document.title = windowLabel;
    const route = isCommentsPage() ? location.pathname : '';
    if (route !== state.route) {
      state.route = route;
      state.seen.clear();
      state.newComments = 0;
      state.lastDetected = 0;
      state.baseline = true;
      if (route) setTimeout(() => { state.baseline = false; scan(); }, 1800);
    }
    const visible = rows();
    const freshComments = [];
    let recognizedComments = 0;
    if (state.route) {
      for (const row of visible) {
        const comment = extractComment(row);
        if (!comment) continue;
        recognizedComments += 1;
        const key = keyFor(row, comment);
        if (state.seen.has(key)) continue;
        state.seen.add(key);
        if (!state.baseline) {
          state.newComments += 1;
          state.lastDetected = Date.now();
          freshComments.push(comment);
        }
        if (state.seen.size > 5000) state.seen.delete(state.seen.values().next().value);
      }
    }
    state.recognizedComments = recognizedComments;
    if (state.route && visible.length > 0 && recognizedComments === 0) {
      state.unrecognizedScans += 1;
      if (state.unrecognizedScans >= 3) state.lastError = 'Facebook muestra filas, pero no se pudo extraer autor y texto.';
    } else if (recognizedComments > 0) {
      state.unrecognizedScans = 0;
      state.lastError = '';
    }
    report(visible.length, freshComments);
  }
  function extractComment(row) {
    const streamId = location.pathname.match(/\/live\/producer\/dashboard\/(\d+)\/comments(?:\/|$)/i)?.[1] || '';
    if (!streamId) return null;
    const aria = normalize(row.getAttribute('aria-label'));
    // This selector only runs on Live Producer's comments route. Own comments
    // may show "Fijar" and omit the "Responder/Comentario" labels.
    const isActionOrTime = value => /^(?:like|reply|responder|respuesta|me gusta|share|compartir|see translation|ver traducci[oó]n|hide|ocultar|send|enviar|pin|fijar|edit|editar|more|m[aá]s|\d+\s*(?:s|sec|min|m|h|d|w|sem|day|days|week|weeks|hora|horas|d[ií]a|d[ií]as)(?:\s+ago|\s+atr[aá]s)?|\d{1,2}:\d{2})$/i.test(value);
    const anchors = [...row.querySelectorAll('a[href]')].map(anchor => ({
      name: normalize(anchor.innerText || anchor.getAttribute('aria-label')),
      href: anchor.href,
      hasImage: !!anchor.querySelector('img')
    })).filter(item => {
      if (!item.name || item.name.length > 100 || isActionOrTime(item.name)) return false;
      try {
        const url = new URL(item.href);
        return (url.hostname === 'facebook.com' || url.hostname.endsWith('.facebook.com')) && url.pathname !== '/';
      } catch (_) { return false; }
    });
    const author = anchors.find(item => item.hasImage) || anchors[0] || null;
    const labelName = aria.match(/^(?:comment|comentario)(?:\s+(?:from|by|de|por))\s+(.+)$/i)?.[1] || '';
    const fallbackName = [...row.querySelectorAll('[dir="auto"]')]
      .map(node => normalize(node.innerText || node.textContent))
      .find(value => value && value.length <= 100 && !isActionOrTime(value));
    const authorName = author?.name || normalize(labelName) || fallbackName || '';
    if (!authorName || isActionOrTime(authorName)) return null;
    function stableParticipantID(rawURL) {
      if (!rawURL) return '';
      try {
        const url = new URL(rawURL);
        if (url.hostname !== 'facebook.com' && !url.hostname.endsWith('.facebook.com')) return '';
        const pathMatch = url.pathname.match(/^\/people\/[^/]+\/(\d+)\/?$/i) || url.pathname.match(/^\/(\d+)\/?$/);
        const candidate = url.searchParams.get('id') || pathMatch?.[1] || '';
        return /^\d{6,32}$/.test(candidate) ? candidate : '';
      } catch (_) { return ''; }
    }
    const participantId = stableParticipantID(author?.href || '');
    const directText = [...row.querySelectorAll('[dir="auto"]')]
      .filter(node => node.getClientRects().length && !node.querySelector('[dir="auto"]'))
      .map(node => {
        const clone = node.cloneNode(true);
        clone.querySelectorAll('img[alt]').forEach(img => img.replaceWith(document.createTextNode(img.getAttribute('alt') || '')));
        return normalize(clone.textContent || node.innerText || node.textContent);
      })
      .filter(value => value && value !== authorName && value.length <= 3500 && !isActionOrTime(value));
    const message = directText.sort((left, right) => right.length - left.length)[0] || '';
    if (!message || message.length > 3500) return null;
    const links = [...row.querySelectorAll('a[href]')].map(anchor => anchor.href);
    const commentLink = links.find(href => /comment_id=|reply_comment_id=|\/comment\//i.test(href)) || '';
    let externalId = '';
    try {
      const url = new URL(commentLink);
      externalId = url.searchParams.get('comment_id') || url.searchParams.get('reply_comment_id') || '';
    } catch (_) {}
    if (!externalId) {
      const dataId = row.getAttribute('data-id') || row.id || '';
      if (/^[0-9_]{6,}$/.test(dataId)) externalId = dataId;
    }
    let eventAt = '';
    const timeNode = row.querySelector('[data-utime], time[datetime], abbr[title]');
    if (timeNode) {
      const epoch = Number(timeNode.getAttribute('data-utime'));
      const parsed = Date.parse(timeNode.getAttribute('datetime') || timeNode.getAttribute('title') || '');
      const milliseconds = Number.isFinite(epoch) && epoch > 0 ? (epoch < 1e12 ? epoch * 1000 : epoch) : parsed;
      if (Number.isFinite(milliseconds) && milliseconds > 0) eventAt = new Date(milliseconds).toISOString();
    }
    return { externalId, participantId, streamId, author: authorName, message, eventAt };
  }
  function schedule() {
    if (!state.timer) state.timer = setTimeout(scan, 500);
  }
  const observer = new MutationObserver(schedule);
  if (document.documentElement) observer.observe(document.documentElement, { childList: true, subtree: true, characterData: true });
  scan();
  setInterval(scan, 5000);
  return true;
})();`
}

func printProbe(label string, probe domProbe) {
	status := "Facebook abierto; espera la vista de comentarios"
	if probe.CommentsPage {
		status = "vista de comentarios encontrada"
	}
	last := "sin comentarios nuevos"
	if probe.LastDetected > 0 {
		last = time.UnixMilli(probe.LastDetected).Local().Format("15:04:05")
	}
	fmt.Printf("[%s] %s | filas: %d | reconocidas: %d | guardadas en sesión: %d | último: %s",
		label, status, probe.VisibleRows, probe.RecognizedComments, probe.NewComments, last)
	if probe.LastError != "" {
		fmt.Printf(" | aviso: %s", probe.LastError)
	}
	fmt.Println()
}

func closeAccounts(accounts []*account) {
	for _, a := range accounts {
		if a.debugURL != "" {
			allocator, cancelAllocator := remote.NewAllocator(context.Background(), a.debugURL)
			closeCtx, cancelTarget := chromedp.NewContext(allocator)
			closeCtx, cancelTimeout := context.WithTimeout(closeCtx, 3*time.Second)
			_, _ = chromedp.CallBrowser(closeCtx, browser.Close, cdp.Empty{})
			cancelTimeout()
			cancelTarget()
			cancelAllocator()
		} else if a.proc != nil && a.proc.Process != nil {
			_ = a.proc.Process.Kill()
		}
		if a.cancel != nil {
			a.cancel()
		}
	}
}

func localWebSocketURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "ws" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func isFacebookURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "facebook.com" || strings.HasSuffix(host, ".facebook.com")
}
