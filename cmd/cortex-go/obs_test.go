package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestOBSProbeCompletesReadOnlyHandshake(t *testing.T) {
	upgrader := websocket.Upgrader{Subprotocols: []string{"obswebsocket.json"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer connection.Close()
		if err := connection.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"rpcVersion": 1}}); err != nil {
			t.Errorf("escribir saludo: %v", err)
			return
		}
		var identify struct {
			Op int `json:"op"`
			D  struct {
				Subscriptions int `json:"eventSubscriptions"`
			} `json:"d"`
		}
		if err := connection.ReadJSON(&identify); err != nil {
			t.Errorf("leer Identify: %v", err)
			return
		}
		if identify.Op != 1 || identify.D.Subscriptions != 0 {
			t.Errorf("OBS recibió algo distinto a una identificación sin eventos: %#v", identify)
			return
		}
		_ = connection.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}})
	}))
	defer server.Close()
	address := "ws" + strings.TrimPrefix(server.URL, "http")
	status := probeOBS(context.Background(), address, "")
	if status.State != "connected" || !status.ReadOnly {
		t.Fatalf("estado OBS inesperado: %#v", status)
	}
}

func TestOBSAuthenticationDerivation(t *testing.T) {
	actual := obsAuthentication("password", "salt", "challenge")
	if actual == "" || actual == "password" || actual == "salt" {
		t.Fatalf("se esperaba un token derivado, se obtuvo %q", actual)
	}
}

func TestOBSProbeDoesNotConnectToNonLocalAddress(t *testing.T) {
	status := probeOBS(context.Background(), "ws://example.com:4455", "never-send-this")
	if status.State != "disconnected" || status.Label != "OBS sin conexión" {
		t.Fatalf("se aceptó una dirección OBS que no es local: %#v", status)
	}
}
