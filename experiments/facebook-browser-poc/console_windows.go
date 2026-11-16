//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func disableConsoleQuickEdit() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getStdHandle := kernel32.NewProc("GetStdHandle")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	handle, _, _ := getStdHandle.Call(^uintptr(9))
	if handle == 0 || handle == ^uintptr(0) {
		return
	}

	var mode uint32
	ok, _, _ := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	if ok == 0 {
		return
	}

	const (
		enableQuickEdit     = 0x0040
		enableExtendedFlags = 0x0080
	)
	mode |= enableExtendedFlags
	mode &^= enableQuickEdit
	_, _, _ = setConsoleMode.Call(handle, uintptr(mode))
}
