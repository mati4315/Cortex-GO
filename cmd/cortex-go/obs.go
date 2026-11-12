package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type obsStatus struct {
	State         string `json:"state"`
	Label         string `json:"label"`
	Detail        string `json:"detail,omitempty"`
	ReadOnly      bool   `json:"readOnly"`
	LastCheckedAt string `json:"lastCheckedAt,omitempty"`
}

type obsProtocolMessage struct {
	Op int `json:"op"`
	D  struct {
		RPCVersion     int `json:"rpcVersion"`
		Authentication *struct {
			Salt      string `json:"salt"`
			Challenge string `json:"challenge"`
		} `json:"authentication"`
	} `json:"d"`
}

func probeOBSFromEnvironment(ctx context.Context) obsStatus {
	address := strings.TrimSpace(os.Getenv("CORTEX_OBS_WS"))
	if address == "" {
		address = "ws://127.0.0.1:4455"
	}
	password := os.Getenv("CORTEX_OBS_PASSWORD")
	return probeOBS(ctx, address, password)
}

func probeOBS(ctx context.Context, address, password string) obsStatus {
	disconnected := func(detail string) obsStatus {
		return obsStatus{State: "disconnected", Label: "OBS sin conexión", Detail: detail, ReadOnly: true}
	}
	if !strings.HasPrefix(address, "ws://") && !strings.HasPrefix(address, "wss://") {
		return disconnected("La dirección local de OBS no es válida")
	}
	parsedAddress, err := url.Parse(address)
	if err != nil || parsedAddress.User != nil || !isLoopbackHost(parsedAddress.Hostname()) {
		return disconnected("Por seguridad, la dirección WebSocket de OBS debe ser local")
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: 2 * time.Second,
		Subprotocols:     []string{"obswebsocket.json"},
	}
	conn, response, err := dialer.DialContext(ctx, address, http.Header{})
	if err != nil {
		if response != nil && response.StatusCode != 0 {
			return disconnected("OBS rechazó la conexión WebSocket")
		}
		return disconnected("No se pudo conectar al WebSocket local de OBS")
	}
	defer conn.Close()
	deadline := time.Now().Add(2500 * time.Millisecond)
	if err := conn.SetReadDeadline(deadline); err != nil {
		return disconnected("No se pudo comprobar la sesión de OBS")
	}
	var hello obsProtocolMessage
	if err := conn.ReadJSON(&hello); err != nil || hello.Op != 0 {
		return disconnected("OBS no respondió con el saludo del protocolo esperado")
	}
	rpcVersion := hello.D.RPCVersion
	if rpcVersion < 1 {
		rpcVersion = 1
	}
	identify := map[string]any{"rpcVersion": rpcVersion, "eventSubscriptions": 0}
	if hello.D.Authentication != nil {
		if password == "" {
			return obsStatus{State: "authentication_required", Label: "OBS requiere clave", Detail: "Configurá la clave local mediante CORTEX_OBS_PASSWORD para comprobar la conexión", ReadOnly: true}
		}
		identify["authentication"] = obsAuthentication(password, hello.D.Authentication.Salt, hello.D.Authentication.Challenge)
	}
	if err := conn.WriteJSON(map[string]any{"op": 1, "d": identify}); err != nil {
		return disconnected("No se pudo iniciar la sesión de solo lectura con OBS")
	}
	var identified obsProtocolMessage
	if err := conn.ReadJSON(&identified); err != nil {
		if isAuthClose(err) {
			return obsStatus{State: "authentication_failed", Label: "Clave de OBS rechazada", Detail: "Revisá la variable local CORTEX_OBS_PASSWORD", ReadOnly: true}
		}
		return disconnected("OBS no completó la sesión WebSocket de solo lectura")
	}
	if identified.Op != 2 {
		return disconnected("OBS no confirmó la sesión WebSocket")
	}
	return obsStatus{State: "connected", Label: "OBS conectado", Detail: "Sesión WebSocket autenticada; Cortex solo consulta el estado de conexión", ReadOnly: true}
}

func obsAuthentication(password, salt, challenge string) string {
	secret := sha256.Sum256([]byte(password + salt))
	secretBase64 := base64.StdEncoding.EncodeToString(secret[:])
	authentication := sha256.Sum256([]byte(secretBase64 + challenge))
	return base64.StdEncoding.EncodeToString(authentication[:])
}

func isAuthClose(err error) bool {
	var closeErr *websocket.CloseError
	return errors.As(err, &closeErr) && closeErr.Code == 4009
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
