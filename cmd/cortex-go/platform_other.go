//go:build !windows

package main

import "os/exec"

func configureHiddenProcess(*exec.Cmd) {}

func openURL(url string) {
	cmd := exec.Command("xdg-open", url)
	_ = cmd.Start()
}
