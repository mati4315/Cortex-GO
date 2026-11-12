//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func configureHiddenProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func openURL(url string) {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	configureHiddenProcess(cmd)
	if err := cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
}
