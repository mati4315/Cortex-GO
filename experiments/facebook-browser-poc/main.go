package main

import (
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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

const (
	firstDebugPort  = 19221
	secondDebugPort = 19222
	bridgeName      = "cortexFacebookProbe"
)

type account struct {
	label    string
	id       string
	feedID   string
	port     int
	proc     *exec.Cmd
	cancel   context.CancelFunc
	debugURL string
	saved    int
}

type debuggerVersion struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type domProbe struct {
	Account      string            `json:"account"`
	CommentsPage bool              `json:"commentsPage"`
	VisibleRows  int               `json:"visibleRows"`
	NewComments  int               `json:"newComments"`
	LastDetected int64             `json:"lastDetected"`
	Comments     []capturedComment `json:"comments"`
}

type capturedComment struct {
	ExternalID string `json:"externalId"`
	StreamID   string `json:"streamId"`
	Author     string `json:"author"`
	Message    string `json:"message"`
	EventAt    string `json:"eventAt"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Prueba Facebook:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("facebook-browser-poc", flag.ContinueOnError)
	browserChoice := flags.String("browser", "edge", "navegador instalado: edge o brave")
	cortexURL := flags.String("cortex-url", "http://127.0.0.1:17845", "URL local del panel Cortex Go")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := validateCortexURL(*cortexURL); err != nil {
		return err
	}

	browserPath, err := findBrowser(strings.ToLower(strings.TrimSpace(*browserChoice)))
	if err != nil {
		return err
	}
	localData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localData == "" {
		return errors.New("Windows no definió LOCALAPPDATA; no se crearán perfiles dentro del repositorio")
	}
	profileRoot := filepath.Join(localData, "CortexGo", "FacebookBrowserPoC")
	if err := os.MkdirAll(profileRoot, 0o700); err != nil {
		return fmt.Errorf("no se pudo preparar el almacenamiento privado local: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	accounts := []*account{
		{label: "Cuenta 1", id: "cuenta-1", feedID: "a", port: firstDebugPort},
		{label: "Cuenta 2", id: "cuenta-2", feedID: "b", port: secondDebugPort},
	}

	fmt.Println("Prueba técnica de dos sesiones Facebook con perfiles aislados")
	fmt.Println("El navegador conserva las cookies en LOCALAPPDATA, fuera del repositorio.")
	fmt.Println("Inicia sesión manualmente en cada ventana y, en esa misma pestaña, abre Live Producer > Comentarios.")
	fmt.Println("Solo envía comentarios nuevos a la base local de Cortex Go; no imprime nombres ni textos en la consola.")
	fmt.Println("Cierra con Ctrl+C para cerrar los dos navegadores de forma ordenada.")

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

func findBrowser(choice string) (string, error) {
	var candidates []string
	switch choice {
	case "edge":
		candidates = []string{
			filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES"), "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "Application", "msedge.exe"),
		}
	case "brave":
		candidates = []string{
			filepath.Join(os.Getenv("PROGRAMFILES"), "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		}
	default:
		return "", fmt.Errorf("navegador desconocido %q; usa -browser edge o -browser brave", choice)
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no encontré %s instalado; usa -browser edge o -browser brave", choice)
}

func launch(ctx context.Context, browserPath, profileRoot string, a *account) error {
	if err := ensurePortFree(a.port); err != nil {
		return fmt.Errorf("el puerto local %d ya está ocupado; cierra la prueba anterior y vuelve a intentarlo", a.port)
	}
	profileDir := filepath.Join(profileRoot, a.id)
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return fmt.Errorf("no se pudo preparar el perfil de %s: %w", a.label, err)
	}

	cmd := exec.Command(browserPath,
		"--user-data-dir="+profileDir,
		"--remote-debugging-address=127.0.0.1",
		"--remote-debugging-port="+strconv.Itoa(a.port),
		"--no-first-run",
		"--no-default-browser-check",
		"https://www.facebook.com/",
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
	return nil
}

func ensurePortFree(port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	return listener.Close()
}

func connectAndObserve(parent context.Context, a *account, cortexURL string) error {
	allocator, cancelAllocator := remote.NewAllocator(parent, a.debugURL)
	ctx, cancelTarget := chromedp.NewContext(allocator)
	a.cancel = func() {
		cancelTarget()
		cancelAllocator()
	}

	if err := chromedp.Do(ctx, chromedp.ExposeFunc(bridgeName, func(_ context.Context, payload string) (chromedp.Void, error) {
		var probe domProbe
		if err := json.Unmarshal([]byte(payload), &probe); err != nil || probe.Account != a.id {
			return chromedp.Void{}, nil
		}
		pageURL, err := chromedp.Run(ctx, chromedp.Location())
		if err != nil || !isFacebookURL(pageURL) {
			return chromedp.Void{}, nil
		}
		if err := postCaptureStatus(parent, cortexURL, a, probe); err != nil {
			fmt.Printf("[%s] No se pudo actualizar el estado de captura.\n", a.label)
		}
		for _, comment := range probe.Comments {
			saved, err := postCapturedComment(parent, cortexURL, a, comment)
			if err != nil {
				fmt.Printf("[%s] No se pudo guardar un comentario (error de conexión o respuesta del panel).\n", a.label)
				continue
			}
			if saved {
				a.saved++
				fmt.Printf("[%s] Comentario nuevo guardado; total de esta sesión: %d.\n", a.label, a.saved)
			} else {
				fmt.Printf("[%s] Comentario repetido descartado por deduplicación.\n", a.label)
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
		"author": comment.Author, "message": comment.Message, "eventAt": comment.EventAt,
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
		"commentsPage": probe.CommentsPage,
		"visibleRows":  probe.VisibleRows,
		"newComments":  probe.NewComments,
		"lastDetected": probe.LastDetected,
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

func observerScript(accountID string) string {
	encoded, _ := json.Marshal(accountID)
	return `(() => {
  const account = ` + string(encoded) + `;
  const windowLabel = account === 'cuenta-1' ? 'Cortex Go — Cuenta 1' : 'Cortex Go — Cuenta 2';
  const marker = '__cortexFacebookProbe_' + account;
  if (window[marker]) return true;
  const state = { route: '', seen: new Set(), baseline: true, newComments: 0, lastDetected: 0, timer: 0, lastReportAt: 0, lastReportSignature: '' };
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
  function keyFor(row) {
    const links = [...row.querySelectorAll('a[href]')].map(node => node.href);
    const commentLink = links.find(href => /comment_id=|reply_comment_id=|\/comment\//i.test(href));
    if (commentLink) return 'id:' + commentLink;
    return 'text:' + hash(normalize(row.innerText || row.textContent));
  }
  function report(visibleRows, comments = []) {
    try {
      const bridge = window.` + bridgeName + `;
      if (typeof bridge !== 'function') return;
      const signature = [isCommentsPage(), visibleRows, state.newComments, state.lastDetected].join(':');
      const now = Date.now();
      if (signature === state.lastReportSignature && now - state.lastReportAt < 5000) return;
      state.lastReportSignature = signature;
      state.lastReportAt = now;
      bridge(JSON.stringify({
        account,
        commentsPage: isCommentsPage(),
        visibleRows,
        newComments: state.newComments,
        lastDetected: state.lastDetected,
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
    if (state.route) {
      for (const row of visible) {
        const key = keyFor(row);
        if (state.seen.has(key)) continue;
        state.seen.add(key);
        if (!state.baseline) {
          state.newComments += 1;
          state.lastDetected = Date.now();
          const comment = extractComment(row);
          if (comment) freshComments.push(comment);
        }
        if (state.seen.size > 5000) state.seen.delete(state.seen.values().next().value);
      }
    }
    report(visible.length, freshComments);
  }
  function extractComment(row) {
    const streamId = location.pathname.match(/\/live\/producer\/dashboard\/(\d+)\/comments(?:\/|$)/i)?.[1] || '';
    if (!streamId) return null;
    const rowText = normalize(row.innerText || row.textContent);
    const aria = normalize(row.getAttribute('aria-label'));
    if (!/(comment|comentario|reply|responder|respuesta)/i.test(aria + ' ' + rowText)) return null;
    const isActionOrTime = value => /^(?:like|reply|responder|respuesta|me gusta|share|compartir|see translation|ver traducci[oó]n|hide|ocultar|send|enviar|\d+\s*(?:s|sec|min|m|h|d|w|sem|day|days|week|weeks|hora|horas|d[ií]a|d[ií]as)(?:\s+ago|\s+atr[aá]s)?|\d{1,2}:\d{2})$/i.test(value);
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
    const authorName = author?.name || normalize(labelName);
    if (!authorName || isActionOrTime(authorName)) return null;
    const directText = [...row.querySelectorAll('[dir="auto"]')]
      .filter(node => node.getClientRects().length && !node.querySelector('[dir="auto"]'))
      .map(node => normalize(node.innerText || node.textContent))
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
    return { externalId, streamId, author: authorName, message, eventAt: '' };
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
	fmt.Printf("[%s] %s | filas visibles: %d | nuevos: %d | último: %s\n",
		label, status, probe.VisibleRows, probe.NewComments, last)
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
