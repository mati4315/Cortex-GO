package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const defaultControlAddress = "127.0.0.1:17845"
const maxGuardianRestarts = 5

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Cortex Go:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("cortex-go", flag.ContinueOnError)
	worker := flags.Bool("worker", false, "modo interno del servidor supervisado")
	managed := flags.Bool("managed", false, "modo interno supervisado por el lanzador de Cortex")
	listenAddress := flags.String("listen", defaultControlAddress, "dirección local del panel")
	dbPath := flags.String("db", "", "ruta interna de la base de prueba")
	parentURL := flags.String("parent-url", "", "URL interna del supervisor; solo para el servidor hijo")
	dataDir := flags.String("data-dir", "", "carpeta local para datos y registros de prueba")
	noBrowser := flags.Bool("no-browser", false, "no abrir el navegador automáticamente")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if *worker {
		if *dbPath == "" {
			return errors.New("falta la ruta interna de la base de prueba")
		}
		return runWorker(*listenAddress, *dbPath, *parentURL)
	}
	if *dataDir == "" {
		*dataDir = defaultDataDirectory()
	}
	if !*managed {
		return runGuardian(args, *dataDir, *listenAddress, !*noBrowser)
	}
	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		return fmt.Errorf("no se pudo preparar la carpeta local: %w", err)
	}
	logDir := filepath.Join(*dataDir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return fmt.Errorf("no se pudo preparar el registro local: %w", err)
	}
	logFile, err := os.OpenFile(filepath.Join(logDir, "cortex-go.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("no se pudo abrir el registro local: %w", err)
	}
	defer logFile.Close()
	logger := log.New(logFile, "", log.Ldate|log.Ltime|log.LUTC)

	if !isLoopbackAddress(*listenAddress) {
		return errors.New("por seguridad, el primer corte solo acepta una dirección local (localhost)")
	}
	controlListener, err := net.Listen("tcp", *listenAddress)
	if err != nil {
		if isExistingCortex(*listenAddress) {
			if !*noBrowser {
				openURL(controlURL(controlListenerAddress(*listenAddress)))
			}
			return nil
		}
		return fmt.Errorf("no se pudo iniciar el panel local en %s: %w", *listenAddress, err)
	}
	defer controlListener.Close()
	controlURLValue := controlURL(controlListener.Addr().String())

	workerAddress, err := reserveWorkerAddress()
	if err != nil {
		return fmt.Errorf("no se pudo preparar el servidor interno: %w", err)
	}
	databasePath := filepath.Join(*dataDir, "first-cut.sqlite")
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo identificar Cortex Go: %w", err)
	}
	manager := newProcessSupervisor(executable, workerAddress, databasePath, controlURLValue, logger, logFile)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.Run(ctx)

	shutdown := make(chan struct{})
	var shutdownOnce sync.Once
	control := newControlServer(manager, func() {
		shutdownOnce.Do(func() { close(shutdown) })
	})
	server := &http.Server{
		Handler:           control,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(controlListener) }()
	logger.Printf("panel local iniciado en %s", controlURLValue)
	if !*noBrowser {
		openURL(controlURLValue)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	var serveFailure error
	select {
	case <-shutdown:
		logger.Print("cierre solicitado desde el panel")
	case sig := <-signals:
		logger.Printf("cierre solicitado por señal: %s", sig)
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("el panel local se cerró: %v", err)
			serveFailure = fmt.Errorf("el panel local se cerró inesperadamente: %w", err)
		}
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	select {
	case <-manager.Done():
	case <-time.After(6 * time.Second):
		logger.Print("el supervisor tardó más de lo esperado en cerrar")
	}
	logger.Print("Cortex Go cerró el supervisor de prueba")
	return serveFailure
}

func runGuardian(args []string, dataDir, listenAddress string, openBrowser bool) error {
	if isExistingCortex(listenAddress) {
		if openBrowser {
			openURL(controlURL(controlListenerAddress(listenAddress)))
		}
		return nil
	}
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return fmt.Errorf("no se pudo preparar el registro del lanzador: %w", err)
	}
	logFile, err := os.OpenFile(filepath.Join(logDir, "cortex-go-guardian.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("no se pudo abrir el registro del lanzador: %w", err)
	}
	defer logFile.Close()
	logger := log.New(logFile, "", log.Ldate|log.Ltime|log.LUTC)
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo identificar Cortex Go para supervisarlo: %w", err)
	}

	restarts := 0
	for {
		childArgs := make([]string, 0, len(args)+1)
		childArgs = append(childArgs, "--managed")
		childArgs = append(childArgs, args...)
		child := exec.Command(executable, childArgs...)
		configureHiddenProcess(child)
		child.Stdout = logFile
		child.Stderr = logFile
		if err := child.Start(); err != nil {
			restarts++
			logger.Printf("no se pudo iniciar Cortex Go (intento %d): %v", restarts, err)
		} else {
			logger.Printf("Cortex Go iniciado por el lanzador (pid %d)", child.Process.Pid)
			if err := child.Wait(); err == nil {
				logger.Print("Cortex Go cerró normalmente; el lanzador terminó")
				return nil
			} else {
				restarts++
				logger.Printf("Cortex Go terminó inesperadamente (intento automático %d): %v", restarts, err)
			}
		}
		if restarts > maxGuardianRestarts {
			logger.Printf("se alcanzó el límite de reinicios; abre Cortex Go de nuevo para continuar")
			return fmt.Errorf("Cortex Go terminó después de %d intentos automáticos", maxGuardianRestarts)
		}
		delay := retryDelay(restarts)
		logger.Printf("se intentará iniciar Cortex Go otra vez en %s", delay)
		time.Sleep(delay)
	}
}

// defaultDataDirectory mantiene los datos de esta prueba bajo el proyecto al
// ejecutarlo desde la raíz o desde bin/; no busca ni abre datos antiguos.
func defaultDataDirectory() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if strings.EqualFold(filepath.Base(dir), "bin") {
			dir = filepath.Dir(dir)
		}
		return filepath.Join(dir, "data")
	}
	return filepath.Join(".", "data")
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func reserveWorkerAddress() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	address := listener.Addr().String()
	return address, listener.Close()
}

func controlURL(address string) string { return "http://" + address }

func controlListenerAddress(configured string) string {
	if strings.HasSuffix(configured, ":0") {
		return defaultControlAddress
	}
	return configured
}

func isExistingCortex(address string) bool {
	target := controlURL(controlListenerAddress(address)) + "/api/status"
	client := &http.Client{Timeout: 700 * time.Millisecond}
	response, err := client.Get(target)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.Header.Get("X-Cortex-Go-Control-Plane") == "1"
}
