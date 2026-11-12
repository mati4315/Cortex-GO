package main

import (
	"net/http/httptest"
	"testing"
)

func TestSameOriginAllowsLocalAndRejectsForeignBrowserRequests(t *testing.T) {
	local := httptest.NewRequest("POST", "http://127.0.0.1:17845/api/worker/restart", nil)
	local.Host = "127.0.0.1:17845"
	local.Header.Set("Origin", "http://127.0.0.1:17845")
	if !sameOrigin(local) {
		t.Fatal("se rechazó una acción del panel local")
	}

	privateHTTPS := httptest.NewRequest("POST", "http://127.0.0.1:17845/api/backup", nil)
	privateHTTPS.Host = "127.0.0.1:17845"
	privateHTTPS.Header.Set("Origin", "https://pc-escritorio.tail983e46.ts.net")
	privateHTTPS.Header.Set("X-Forwarded-Host", "pc-escritorio.tail983e46.ts.net")
	privateHTTPS.Header.Set("X-Forwarded-Proto", "https")
	privateHTTPS.Header.Set("Tailscale-User-Login", "mati@example.com")
	if !sameOrigin(privateHTTPS) {
		t.Fatal("se rechazó una acción legítima recibida por Tailscale Serve")
	}

	spoofedHTTPS := httptest.NewRequest("POST", "http://127.0.0.1:17845/api/backup", nil)
	spoofedHTTPS.Host = "127.0.0.1:17845"
	spoofedHTTPS.Header.Set("Origin", "https://pc-escritorio.tail983e46.ts.net")
	spoofedHTTPS.Header.Set("X-Forwarded-Host", "pc-escritorio.tail983e46.ts.net")
	spoofedHTTPS.Header.Set("X-Forwarded-Proto", "https")
	if sameOrigin(spoofedHTTPS) {
		t.Fatal("se aceptó una cabecera HTTPS reenviada sin identidad Tailscale")
	}

	foreign := httptest.NewRequest("POST", "http://127.0.0.1:17845/api/worker/restart", nil)
	foreign.Host = "127.0.0.1:17845"
	foreign.Header.Set("Origin", "http://sitio-ajeno.example")
	if sameOrigin(foreign) {
		t.Fatal("se aceptó una acción enviada desde otro origen")
	}
}
