package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
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

type obsResponseMessage struct {
	Op int `json:"op"`
	D  struct {
		RequestID     string `json:"requestId"`
		RequestStatus struct {
			Result  bool   `json:"result"`
			Code    int    `json:"code"`
			Comment string `json:"comment"`
		} `json:"requestStatus"`
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

func switchOBSSceneFromEnvironment(ctx context.Context, sceneName string) error {
	address := strings.TrimSpace(os.Getenv("CORTEX_OBS_WS"))
	if address == "" {
		address = "ws://127.0.0.1:4455"
	}
	return switchOBSScene(ctx, address, os.Getenv("CORTEX_OBS_PASSWORD"), sceneName)
}

func switchOBSScene(ctx context.Context, address, password, sceneName string) error {
	sceneName = strings.TrimSpace(sceneName)
	if sceneName == "" || len([]rune(sceneName)) > 128 {
		return errors.New("el nombre de escena de OBS no es válido")
	}
	if !strings.HasPrefix(address, "ws://") && !strings.HasPrefix(address, "wss://") {
		return errors.New("la dirección local de OBS no es válida")
	}
	parsedAddress, err := url.Parse(address)
	if err != nil || parsedAddress.User != nil || !isLoopbackHost(parsedAddress.Hostname()) {
		return errors.New("por seguridad, la dirección WebSocket de OBS debe ser local")
	}
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second, Subprotocols: []string{"obswebsocket.json"}}
	conn, response, err := dialer.DialContext(ctx, address, http.Header{})
	if err != nil {
		if response != nil && response.StatusCode != 0 {
			return errors.New("OBS rechazó la conexión WebSocket")
		}
		return errors.New("no se pudo conectar al WebSocket local de OBS")
	}
	defer conn.Close()
	conn.SetReadLimit(64 << 10)
	deadline := time.Now().Add(4 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return errors.New("no se pudo iniciar la sesión de OBS")
	}
	var hello obsProtocolMessage
	if err := conn.ReadJSON(&hello); err != nil || hello.Op != 0 {
		return errors.New("OBS no respondió con el saludo del protocolo esperado")
	}
	rpcVersion := hello.D.RPCVersion
	if rpcVersion < 1 {
		rpcVersion = 1
	}
	identify := map[string]any{"rpcVersion": rpcVersion, "eventSubscriptions": 0}
	if hello.D.Authentication != nil {
		if password == "" {
			return errors.New("OBS requiere configurar CORTEX_OBS_PASSWORD localmente")
		}
		identify["authentication"] = obsAuthentication(password, hello.D.Authentication.Salt, hello.D.Authentication.Challenge)
	}
	if err := conn.WriteJSON(map[string]any{"op": 1, "d": identify}); err != nil {
		return errors.New("no se pudo iniciar la sesión de OBS")
	}
	var identified obsProtocolMessage
	if err := conn.ReadJSON(&identified); err != nil {
		if isAuthClose(err) {
			return errors.New("OBS rechazó la clave configurada localmente")
		}
		return errors.New("OBS no completó la sesión WebSocket")
	}
	if identified.Op != 2 {
		return errors.New("OBS no confirmó la sesión WebSocket")
	}
	requestID := fmt.Sprintf("cortex-go-scene-%d", time.Now().UnixNano())
	if err := conn.WriteJSON(map[string]any{
		"op": 6,
		"d": map[string]any{
			"requestType": "SetCurrentProgramScene",
			"requestId":   requestID,
			"requestData": map[string]string{"sceneName": sceneName},
		},
	}); err != nil {
		return errors.New("no se pudo enviar la transición a OBS")
	}
	for count := 0; count < 8; count++ {
		var reply obsResponseMessage
		if err := conn.ReadJSON(&reply); err != nil {
			return errors.New("OBS no confirmó la transición solicitada")
		}
		if reply.Op != 7 || reply.D.RequestID != requestID {
			continue
		}
		if !reply.D.RequestStatus.Result {
			return errors.New("OBS no pudo cambiar a la escena indicada; comprueba el nombre exacto")
		}
		return nil
	}
	return errors.New("OBS no respondió a la transición solicitada")
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
