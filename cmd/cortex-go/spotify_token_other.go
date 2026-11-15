//go:build !windows

package main

import "errors"

func spotifyTokenVaultAvailable() bool { return false }
func loadSpotifyRefreshToken() (string, error) {
	return "", errors.New("el almacén seguro requiere Windows")
}
func saveSpotifyRefreshToken(string) error { return errors.New("el almacén seguro requiere Windows") }
func deleteSpotifyRefreshToken() error     { return errors.New("el almacén seguro requiere Windows") }
