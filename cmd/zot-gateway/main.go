// zot-gateway exposes the embedded zot runtime to browser and remote React Native clients.
package main

import (
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	bridge "github.com/patriceckhart/zot-native-sdk/bridge"
)

type request struct {
	ID           any    `json:"id"`
	Method       string `json:"method"`
	RPCToken     string `json:"rpc_token"`
	SessionID    string `json:"session_id"`
	Provider     string `json:"provider"`
	APIKey       string `json:"api_key"`
	AccessToken  string `json:"access_token"`
	AccountID    string `json:"account_id"`
	Model        string `json:"model"`
	SystemPrompt string `json:"system_prompt"`
	Message      string `json:"message"`
	IDToken      string `json:"id_token"`
	History      string `json:"history"`
}

type gateway struct {
	token   string
	origins map[string]bool
}

type connection struct {
	mu       sync.RWMutex
	writeMu  sync.Mutex
	runWG    sync.WaitGroup
	sessions map[string]*bridge.Session
	socket   *websocket.Conn
	token    string
}

func main() {
	address := flag.String("addr", envOr("ZOT_GATEWAY_ADDR", "127.0.0.1:8787"), "HTTP listen address")
	path := flag.String("path", envOr("ZOT_GATEWAY_PATH", "/v1/zot"), "WebSocket endpoint")
	flag.Parse()

	g := &gateway{token: os.Getenv("ZOT_GATEWAY_TOKEN"), origins: originSet(os.Getenv("ZOT_GATEWAY_ORIGINS"))}
	mux := http.NewServeMux()
	mux.HandleFunc(*path, g.serveWebSocket)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	server := &http.Server{Addr: *address, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("zot gateway listening on %s%s", *address, *path)
	if g.token == "" {
		log.Printf("warning: ZOT_GATEWAY_TOKEN is not configured")
	}
	log.Fatal(server.ListenAndServe())
}

func (g *gateway) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  16 * 1024,
		WriteBufferSize: 16 * 1024,
		CheckOrigin: func(request *http.Request) bool {
			return g.originAllowed(request)
		},
	}
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &connection{sessions: make(map[string]*bridge.Session), socket: socket, token: g.token}
	client.run()
}

func (g *gateway) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if len(g.origins) > 0 {
		return g.origins[origin] || g.origins[parsed.Host]
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func (c *connection) run() {
	defer c.close()
	c.socket.SetReadLimit(20 * 1024 * 1024)
	for {
		var req request
		if err := c.socket.ReadJSON(&req); err != nil {
			return
		}
		if !tokenEqual(c.token, req.RPCToken) {
			c.respond(req.ID, nil, fmt.Errorf("unauthorized"))
			continue
		}
		c.handle(req)
	}
}

func (c *connection) handle(req request) {
	switch req.Method {
	case "create_session":
		if req.SessionID == "" {
			c.respond(req.ID, nil, fmt.Errorf("missing session_id"))
			return
		}
		var session *bridge.Session
		var err error
		if req.AccessToken != "" {
			session, err = bridge.NewSessionWithOAuth(req.Provider, req.AccessToken, req.AccountID, req.Model, req.SystemPrompt)
		} else {
			session, err = bridge.NewSession(req.Provider, req.APIKey, req.Model, req.SystemPrompt)
		}
		if err == nil {
			c.mu.Lock()
			previous := c.sessions[req.SessionID]
			c.sessions[req.SessionID] = session
			c.mu.Unlock()
			if previous != nil {
				previous.Abort()
			}
		}
		c.respond(req.ID, map[string]any{"session_id": req.SessionID}, err)
	case "prompt":
		session := c.session(req.SessionID)
		if session == nil {
			c.respond(req.ID, nil, fmt.Errorf("unknown session: %s", req.SessionID))
			return
		}
		c.respond(req.ID, map[string]any{"started": true}, nil)
		c.runWG.Add(1)
		go func() {
			defer c.runWG.Done()
			session.Prompt(req.Message, &stream{connection: c, sessionID: req.SessionID, session: session})
		}()
	case "abort":
		session := c.session(req.SessionID)
		if session != nil {
			session.Abort()
		}
		c.respond(req.ID, map[string]any{"aborted": session != nil}, nil)
	case "close_session":
		c.mu.Lock()
		session := c.sessions[req.SessionID]
		delete(c.sessions, req.SessionID)
		c.mu.Unlock()
		if session != nil {
			session.Abort()
		}
		c.respond(req.ID, map[string]any{"closed": session != nil}, nil)
	case "export_history":
		session := c.session(req.SessionID)
		if session == nil {
			c.respond(req.ID, nil, fmt.Errorf("unknown session: %s", req.SessionID))
			return
		}
		c.respond(req.ID, map[string]any{"history": session.ExportHistory()}, nil)
	case "import_history":
		session := c.session(req.SessionID)
		if session == nil {
			c.respond(req.ID, nil, fmt.Errorf("unknown session: %s", req.SessionID))
			return
		}
		err := session.ImportHistory(req.History)
		c.respond(req.ID, map[string]any{"imported": err == nil}, err)
	case "extract_openai_account_id":
		c.respond(req.ID, map[string]any{"account_id": bridge.ExtractOpenAIAccountID(req.IDToken)}, nil)
	default:
		c.respond(req.ID, nil, fmt.Errorf("unknown method: %s", req.Method))
	}
}

func (c *connection) session(id string) *bridge.Session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessions[id]
}

func (c *connection) respond(id, result any, err error) {
	response := map[string]any{"id": id}
	if err != nil {
		response["error"] = err.Error()
	} else {
		response["result"] = result
	}
	c.write(response)
}

func (c *connection) write(value any) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.socket.WriteJSON(value)
}

func (c *connection) close() {
	c.mu.Lock()
	sessions := c.sessions
	c.sessions = make(map[string]*bridge.Session)
	c.mu.Unlock()
	for _, session := range sessions {
		session.Abort()
	}
	c.runWG.Wait()
	_ = c.socket.Close()
}

type stream struct {
	connection *connection
	sessionID  string
	session    *bridge.Session
}

func (s *stream) event(kind string, payload any) {
	s.connection.write(map[string]any{"event": kind, "session_id": s.sessionID, "payload": payload})
}
func (s *stream) OnText(delta string) { s.event("text", map[string]any{"delta": delta}) }
func (s *stream) OnEvent(kind, payload string) {
	var decoded any
	if json.Unmarshal([]byte(payload), &decoded) != nil {
		decoded = payload
	}
	s.event(kind, decoded)
}
func (s *stream) OnError(message string) { s.event("error", map[string]any{"message": message}) }
func (s *stream) OnDone() {
	s.event("history", map[string]any{"history": s.session.ExportHistory()})
	s.event("done", map[string]any{})
}

func tokenEqual(expected, actual string) bool {
	if expected == "" {
		return true
	}
	if len(expected) != len(actual) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func originSet(value string) map[string]bool {
	result := make(map[string]bool)
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result[item] = true
		}
	}
	return result
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
