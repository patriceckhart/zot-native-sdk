package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestTokenEqual(t *testing.T) {
	for _, test := range []struct {
		expected string
		actual   string
		want     bool
	}{
		{"", "", true},
		{"", "anything", true},
		{"secret", "secret", true},
		{"secret", "wrong", false},
		{"secret", "short", false},
	} {
		if got := tokenEqual(test.expected, test.actual); got != test.want {
			t.Fatalf("tokenEqual(%q, %q) = %v, want %v", test.expected, test.actual, got, test.want)
		}
	}
}

func TestGatewayAuthorization(t *testing.T) {
	g := &gateway{token: "secret", origins: map[string]bool{}}
	server := httptest.NewServer(http.HandlerFunc(g.serveWebSocket))
	defer server.Close()

	socket, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()

	if err := socket.WriteJSON(request{ID: 1, Method: "unknown", RPCToken: "wrong"}); err != nil {
		t.Fatal(err)
	}
	var unauthorized map[string]any
	if err := socket.ReadJSON(&unauthorized); err != nil {
		t.Fatal(err)
	}
	if unauthorized["error"] != "unauthorized" {
		t.Fatalf("unexpected unauthorized response: %#v", unauthorized)
	}

	if err := socket.WriteJSON(request{ID: 2, Method: "unknown", RPCToken: "secret"}); err != nil {
		t.Fatal(err)
	}
	var authorized map[string]any
	if err := socket.ReadJSON(&authorized); err != nil {
		t.Fatal(err)
	}
	if authorized["error"] != "unknown method: unknown" {
		t.Fatalf("unexpected authorized response: %#v", authorized)
	}
}

func TestOriginPolicy(t *testing.T) {
	g := &gateway{origins: originSet("https://app.example.com,localhost:3000")}
	request := httptest.NewRequest(http.MethodGet, "http://api.example.com/v1/zot", nil)

	request.Header.Set("Origin", "https://app.example.com")
	if !g.originAllowed(request) {
		t.Fatal("configured origin was rejected")
	}
	request.Header.Set("Origin", "https://evil.example.com")
	if g.originAllowed(request) {
		t.Fatal("unconfigured origin was accepted")
	}
}
