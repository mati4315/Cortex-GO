//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func spotifyTokenVaultAvailable() bool { return true }

func spotifyTokenPath() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "Cortex Go", "spotify-refresh.dpapi"), nil
}

func protectSpotifyToken(token []byte) ([]byte, error) {
	if len(token) == 0 {
		return nil, errors.New("token vacío")
	}
	input := windows.DataBlob{Size: uint32(len(token)), Data: &token[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(output.Data))))
	return append([]byte(nil), unsafe.Slice(output.Data, int(output.Size))...), nil
}

func unprotectSpotifyToken(token []byte) ([]byte, error) {
	if len(token) == 0 {
		return nil, errors.New("archivo de token vacío")
	}
	input := windows.DataBlob{Size: uint32(len(token)), Data: &token[0]}
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(output.Data))))
	return append([]byte(nil), unsafe.Slice(output.Data, int(output.Size))...), nil
}

func loadSpotifyRefreshToken() (string, error) {
	path, err := spotifyTokenPath()
	if err != nil {
		return "", err
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(encoded) > 16*1024 {
		return "", errors.New("archivo de token demasiado grande")
	}
	plain, err := unprotectSpotifyToken(encoded)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func saveSpotifyRefreshToken(token string) error {
	path, err := spotifyTokenPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	encoded, err := protectSpotifyToken([]byte(token))
	if err != nil {
		return err
	}
	temporary := path + ".new"
	if err := os.WriteFile(temporary, encoded, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func deleteSpotifyRefreshToken() error {
	path, err := spotifyTokenPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
