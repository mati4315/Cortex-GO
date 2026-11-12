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

	foreign := httptest.NewRequest("POST", "http://127.0.0.1:17845/api/worker/restart", nil)
	foreign.Host = "127.0.0.1:17845"
	foreign.Header.Set("Origin", "http://sitio-ajeno.example")
	if sameOrigin(foreign) {
		t.Fatal("se aceptó una acción enviada desde otro origen")
	}
}
