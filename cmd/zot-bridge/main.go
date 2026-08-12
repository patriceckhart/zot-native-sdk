// zot-bridge is a newline-delimited JSON RPC sidecar for desktop SDKs.
package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	bridge "github.com/patriceckhart/zot-native-sdk/bridge"
)

type request struct {
	ID           any    `json:"id"`
	Method       string `json:"method"`
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

type session interface {
	Prompt(message string, stream bridge.Stream)
	ExportHistory() string
	ImportHistory(history string) error
	Abort()
}

type server struct {
	mu        sync.RWMutex
	writeMu   sync.Mutex
	runWG     sync.WaitGroup
	sessions  map[string]session
	out       io.Writer
	newAPIKey func(provider, apiKey, model, systemPrompt string) (session, error)
	newOAuth  func(provider, accessToken, accountID, model, systemPrompt string) (session, error)
}

func newServer(out io.Writer) *server {
	return &server{
		sessions: make(map[string]session),
		out:      out,
		newAPIKey: func(provider, apiKey, model, systemPrompt string) (session, error) {
			return bridge.NewSession(provider, apiKey, model, systemPrompt)
		},
		newOAuth: func(provider, accessToken, accountID, model, systemPrompt string) (session, error) {
			return bridge.NewSessionWithOAuth(provider, accessToken, accountID, model, systemPrompt)
		},
	}
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "oneshot" || os.Args[1] == "oneshot-native") {
		runOneShot(os.Args[1] == "oneshot-native")
		return
	}
	s := newServer(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 20*1024*1024)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			s.write(map[string]any{"error": "invalid request: " + err.Error()})
			continue
		}
		s.handle(req)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	s.closeAll()
	s.runWG.Wait()
}

func runOneShot(nativeOutput bool) {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 20*1024*1024+1))
	out := newServer(os.Stdout)
	fail := func(message string) {
		if nativeOutput {
			stream := &nativeStream{out: os.Stdout}
			stream.line("error", message)
			stream.line("done")
		} else {
			out.write(map[string]any{"event": "error", "session_id": "oneshot", "payload": map[string]any{"message": message}})
			out.write(map[string]any{"event": "done", "session_id": "oneshot", "payload": map[string]any{}})
		}
	}
	if err != nil || len(data) > 20*1024*1024 {
		fail("invalid oneshot input")
		return
	}
	fields := bytes.Split(data, []byte{0})
	if len(fields) != 8 {
		fail("oneshot input requires 8 fields")
		return
	}
	text := func(index int) string { return string(fields[index]) }
	var session *bridge.Session
	if text(2) != "" {
		session, err = bridge.NewSessionWithOAuth(text(0), text(2), text(3), text(4), text(5))
	} else {
		session, err = bridge.NewSession(text(0), text(1), text(4), text(5))
	}
	if err == nil && text(6) != "" {
		err = session.ImportHistory(text(6))
	}
	if err != nil {
		fail(err.Error())
		return
	}
	if nativeOutput {
		session.Prompt(text(7), &nativeStream{out: os.Stdout, session: session})
	} else {
		session.Prompt(text(7), &stream{server: out, sessionID: "oneshot", session: session})
	}
}

func (s *server) handle(req request) {
	switch req.Method {
	case "create_session":
		if req.SessionID == "" {
			s.respond(req.ID, nil, fmt.Errorf("missing session_id"))
			return
		}
		var session session
		var err error
		if req.AccessToken != "" {
			session, err = s.newOAuth(req.Provider, req.AccessToken, req.AccountID, req.Model, req.SystemPrompt)
		} else {
			session, err = s.newAPIKey(req.Provider, req.APIKey, req.Model, req.SystemPrompt)
		}
		if err == nil {
			s.mu.Lock()
			previous := s.sessions[req.SessionID]
			s.sessions[req.SessionID] = session
			s.mu.Unlock()
			if previous != nil {
				previous.Abort()
			}
		}
		s.respond(req.ID, map[string]any{"session_id": req.SessionID}, err)
	case "prompt":
		session := s.session(req.SessionID)
		if session == nil {
			s.respond(req.ID, nil, fmt.Errorf("unknown session: %s", req.SessionID))
			return
		}
		s.respond(req.ID, map[string]any{"started": true}, nil)
		s.runWG.Add(1)
		go func() {
			defer s.runWG.Done()
			session.Prompt(req.Message, &stream{server: s, sessionID: req.SessionID, session: session})
		}()
	case "import_history":
		session := s.session(req.SessionID)
		if session == nil {
			s.respond(req.ID, nil, fmt.Errorf("unknown session: %s", req.SessionID))
			return
		}
		err := session.ImportHistory(req.History)
		s.respond(req.ID, map[string]any{"imported": err == nil}, err)
	case "export_history":
		session := s.session(req.SessionID)
		if session == nil {
			s.respond(req.ID, nil, fmt.Errorf("unknown session: %s", req.SessionID))
			return
		}
		s.respond(req.ID, map[string]any{"history": session.ExportHistory()}, nil)
	case "abort":
		session := s.session(req.SessionID)
		if session != nil {
			session.Abort()
		}
		s.respond(req.ID, map[string]any{"aborted": session != nil}, nil)
	case "close_session":
		s.mu.Lock()
		session := s.sessions[req.SessionID]
		delete(s.sessions, req.SessionID)
		s.mu.Unlock()
		if session != nil {
			session.Abort()
		}
		s.respond(req.ID, map[string]any{"closed": session != nil}, nil)
	case "extract_openai_account_id":
		s.respond(req.ID, map[string]any{"account_id": bridge.ExtractOpenAIAccountID(req.IDToken)}, nil)
	default:
		s.respond(req.ID, nil, fmt.Errorf("unknown method: %s", req.Method))
	}
}

func (s *server) session(id string) session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

func (s *server) closeAll() {
	s.mu.Lock()
	sessions := s.sessions
	s.sessions = make(map[string]session)
	s.mu.Unlock()
	for _, session := range sessions {
		session.Abort()
	}
}

func (s *server) respond(id any, result any, err error) {
	response := map[string]any{"id": id}
	if err != nil {
		response["error"] = err.Error()
	} else {
		response["result"] = result
	}
	s.write(response)
}

func (s *server) write(value any) {
	data, _ := json.Marshal(value)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	fmt.Fprintln(s.out, string(data))
}

type stream struct {
	server    *server
	sessionID string
	session   session
}

func (s *stream) event(kind string, payload any) {
	s.server.write(map[string]any{"event": kind, "session_id": s.sessionID, "payload": payload})
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

type nativeStream struct {
	mu      sync.Mutex
	out     io.Writer
	session session
}

func (s *nativeStream) line(kind string, values ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprint(s.out, kind)
	for _, value := range values {
		fmt.Fprint(s.out, " ", hex.EncodeToString([]byte(value)))
	}
	fmt.Fprintln(s.out)
}
func (s *nativeStream) OnText(delta string)          { s.line("text", delta) }
func (s *nativeStream) OnEvent(kind, payload string) { s.line("event", kind, payload) }
func (s *nativeStream) OnError(message string)       { s.line("error", message) }
func (s *nativeStream) OnDone() {
	s.line("history", s.session.ExportHistory())
	s.line("done")
}
