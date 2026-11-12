package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

const maxAutomaticRestarts = 5

type supervisorSnapshot struct {
	State            string `json:"state"`
	RestartCount     int    `json:"restartCount"`
	LastExitReason   string `json:"lastExitReason,omitempty"`
	LastStartedAt    string `json:"lastStartedAt,omitempty"`
	AutomaticRetries int    `json:"automaticRetries"`
}

type processSupervisor struct {
	executable string
	workerAddr string
	dbPath     string
	parentURL  string
	logger     *log.Logger
	output     *os.File

	mu        sync.RWMutex
	state     supervisorSnapshot
	current   *exec.Cmd
	restartCh chan struct{}
	done      chan struct{}
}

func newProcessSupervisor(executable, workerAddr, dbPath, parentURL string, logger *log.Logger, output *os.File) *processSupervisor {
	return &processSupervisor{
		executable: executable,
		workerAddr: workerAddr,
		dbPath:     dbPath,
		parentURL:  parentURL,
		logger:     logger,
		output:     output,
		restartCh:  make(chan struct{}, 1),
		done:       make(chan struct{}),
		state:      supervisorSnapshot{State: "starting"},
	}
}

func (s *processSupervisor) Done() <-chan struct{} { return s.done }

func (s *processSupervisor) Snapshot() supervisorSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *processSupervisor) WorkerURL() string { return "http://" + s.workerAddr }

func (s *processSupervisor) RequestRestart() {
	s.mu.Lock()
	s.state.LastExitReason = "reinicio manual solicitado desde el panel"
	s.mu.Unlock()
	select {
	case s.restartCh <- struct{}{}:
	default:
	}
}

func (s *processSupervisor) Run(ctx context.Context) {
	defer close(s.done)
	failures := 0
runLoop:
	for {
		select {
		case <-ctx.Done():
			s.setState("stopping", failures)
			return
		default:
		}

		cmd := exec.Command(s.executable, "--worker", "--listen", s.workerAddr, "--db", s.dbPath, "--parent-url", s.parentURL)
		configureHiddenProcess(cmd)
		cmd.Stdout = s.output
		cmd.Stderr = s.output
		if err := cmd.Start(); err != nil {
			failures++
			s.setExit("no se pudo iniciar el servidor interno", failures)
			s.logger.Printf("no se pudo iniciar el servidor interno (intento %d): %v", failures, err)
			if !s.waitBeforeRetry(ctx, failures) {
				return
			}
			continue
		}

		s.mu.Lock()
		s.current = cmd
		s.state.State = "starting"
		s.state.AutomaticRetries = failures
		s.mu.Unlock()
		s.logger.Printf("proceso del servidor interno iniciado (pid %d); esperando comprobación de salud", cmd.Process.Pid)

		waitCh := make(chan error, 1)
		go func() { waitCh <- cmd.Wait() }()
		healthTicker := time.NewTicker(time.Second)
		healthFailures := 0
		for {
			select {
			case err := <-waitCh:
				healthTicker.Stop()
				s.clearCurrent(cmd)
				if ctx.Err() != nil {
					return
				}
				failures++
				reason := "el servidor interno terminó inesperadamente"
				if err != nil {
					reason = "el servidor interno terminó con error"
				}
				s.setExit(reason, failures)
				s.logger.Printf("%s (intento automático %d de %d)", reason, failures, maxAutomaticRestarts)
				if !s.waitBeforeRetry(ctx, failures) {
					return
				}
				continue runLoop
			case <-s.restartCh:
				healthTicker.Stop()
				s.stopCurrent(cmd, waitCh)
				failures = 0
				s.mu.Lock()
				s.state.RestartCount++
				s.state.LastExitReason = "servidor interno reiniciado manualmente"
				s.mu.Unlock()
				s.logger.Print("servidor interno reiniciado manualmente desde el panel")
				continue runLoop
			case <-ctx.Done():
				healthTicker.Stop()
				s.stopCurrent(cmd, waitCh)
				s.setState("stopped", failures)
				return
			case <-healthTicker.C:
				if s.workerHealthy() {
					healthFailures = 0
					if s.Snapshot().State != "running" {
						failures = 0
						s.mu.Lock()
						s.state.State = "running"
						s.state.AutomaticRetries = 0
						s.state.LastStartedAt = time.Now().UTC().Format(time.RFC3339)
						s.mu.Unlock()
						s.logger.Printf("servidor interno saludable (pid %d)", cmd.Process.Pid)
					}
					continue
				}
				healthFailures++
				if healthFailures < 3 {
					continue
				}
				healthTicker.Stop()
				reason := "el servidor interno no respondió a tres comprobaciones de salud"
				s.logger.Printf("%s; se reinicia el proceso", reason)
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				<-waitCh
				s.clearCurrent(cmd)
				failures++
				s.setExit(reason, failures)
				if !s.waitBeforeRetry(ctx, failures) {
					return
				}
				continue runLoop
			}
		}
	}
}

func (s *processSupervisor) workerHealthy() bool {
	client := &http.Client{Timeout: 700 * time.Millisecond}
	response, err := client.Get(s.WorkerURL() + "/api/status")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func (s *processSupervisor) clearCurrent(cmd *exec.Cmd) {
	s.mu.Lock()
	if s.current == cmd {
		s.current = nil
	}
	s.mu.Unlock()
}

func (s *processSupervisor) setState(state string, failures int) {
	s.mu.Lock()
	s.state.State = state
	s.state.AutomaticRetries = failures
	s.mu.Unlock()
}

func (s *processSupervisor) setExit(reason string, failures int) {
	s.mu.Lock()
	s.state.RestartCount++
	s.state.LastExitReason = reason
	s.state.AutomaticRetries = failures
	if failures > maxAutomaticRestarts {
		s.state.State = "needs_attention"
	} else {
		s.state.State = "restarting"
	}
	s.mu.Unlock()
}

func (s *processSupervisor) waitBeforeRetry(ctx context.Context, failures int) bool {
	if failures > maxAutomaticRestarts {
		s.setState("needs_attention", failures)
		s.logger.Print("se alcanzó el límite de reinicios automáticos; se espera una acción manual")
		select {
		case <-s.restartCh:
			s.mu.Lock()
			s.state.AutomaticRetries = 0
			s.state.LastExitReason = "reintento manual solicitado desde el panel"
			s.mu.Unlock()
			return true
		case <-ctx.Done():
			return false
		}
	}
	if failures == 0 {
		failures = 1
	}
	delay := retryDelay(failures)
	s.setState("restarting", failures)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-s.restartCh:
		return true
	case <-ctx.Done():
		return false
	}
}

func retryDelay(failure int) time.Duration {
	if failure < 1 {
		failure = 1
	}
	if failure > 4 {
		failure = 4
	}
	return time.Second * time.Duration(1<<(failure-1))
}

func (s *processSupervisor) stopCurrent(cmd *exec.Cmd, waitCh <-chan error) {
	s.mu.Lock()
	if s.current == cmd {
		s.current = nil
	}
	s.mu.Unlock()
	client := &http.Client{Timeout: 2 * time.Second}
	request, _ := http.NewRequest(http.MethodPost, s.WorkerURL()+"/internal/shutdown", nil)
	if request != nil {
		if response, err := client.Do(request); err == nil {
			_ = response.Body.Close()
		}
	}
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	select {
	case <-waitCh:
		return
	case <-timer.C:
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-waitCh
	}
}
