// Package bridge exposes a small, gomobile-compatible API around zot's agent runtime.
package zot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	zotagent "github.com/patriceckhart/zot/packages/agent"
	"github.com/patriceckhart/zot/packages/core"
	"github.com/patriceckhart/zot/packages/provider"
	providerauth "github.com/patriceckhart/zot/packages/provider/auth"
)

// Stream receives one turn's streaming output. All methods are called serially.
type Stream interface {
	OnText(delta string)
	OnEvent(kind string, payload string)
	OnError(message string)
	OnDone()
}

// Session is a stateful conversation. Prompt calls are serialized.
type Session struct {
	stateMu sync.Mutex
	runMu   sync.Mutex
	agent   *core.Agent
	cancel  context.CancelFunc
	runID   int64
}

// NewSession creates a session using a provider API key.
func NewSession(providerName, apiKey, model, systemPrompt string) (*Session, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("missing API key")
	}
	client, defaultModel, err := newAPIKeyClient(providerName, apiKey)
	if err != nil {
		return nil, err
	}
	return newSession(client, model, defaultModel, systemPrompt), nil
}

// NewSessionWithOAuth creates a session using subscription OAuth credentials.
func NewSessionWithOAuth(providerName, accessToken, accountID, model, systemPrompt string) (*Session, error) {
	client, defaultModel, err := newOAuthClient(providerName, accessToken, accountID)
	if err != nil {
		return nil, err
	}
	return newSession(client, model, defaultModel, systemPrompt), nil
}

// ExtractOpenAIAccountID extracts chatgpt_account_id from an OpenAI id_token.
func ExtractOpenAIAccountID(idToken string) string {
	return providerauth.ExtractOpenAIAccountID(idToken)
}

func newSession(client provider.Client, model, defaultModel, systemPrompt string) *Session {
	if strings.TrimSpace(model) == "" {
		model = defaultModel
	}
	cwd, _ := os.Getwd()
	system := zotagent.BuildSystemPrompt(zotagent.SystemPromptOpts{CWD: cwd, Custom: strings.TrimSpace(systemPrompt)})
	agent := core.NewAgent(client, model, system, core.NewRegistry())
	agent.MaxSteps = 8
	return &Session{agent: agent}
}

// Prompt sends a message and blocks until the turn finishes. Call it off the UI thread.
func (s *Session) Prompt(message string, stream Stream) {
	if stream == nil {
		return
	}
	s.runMu.Lock()

	s.stateMu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	s.runID++
	runID := s.runID
	s.cancel = cancel
	s.stateMu.Unlock()

	err := s.agent.Prompt(ctx, message, nil, func(ev core.AgentEvent) { sendEvent(stream, ev) })
	if err != nil && ctx.Err() == nil {
		stream.OnError(err.Error())
	}

	s.stateMu.Lock()
	if s.runID == runID {
		s.cancel = nil
	}
	s.stateMu.Unlock()
	cancel()
	s.runMu.Unlock()
	stream.OnDone()
}

// ExportHistory returns the provider-neutral conversation transcript as JSON.
func (s *Session) ExportHistory() string {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	data, err := json.Marshal(s.agent.Messages())
	if err != nil {
		return "[]"
	}
	return string(data)
}

// ImportHistory replaces the conversation transcript from ExportHistory JSON.
func (s *Session) ImportHistory(history string) error {
	if len(history) > 16*1024*1024 {
		return fmt.Errorf("history exceeds 16 MiB")
	}
	var messages []provider.Message
	if err := json.Unmarshal([]byte(history), &messages); err != nil {
		return fmt.Errorf("invalid history: %w", err)
	}
	if messages == nil {
		return fmt.Errorf("invalid history: expected a JSON array")
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.agent.SetMessages(messages)
	return nil
}

// Abort cancels the active prompt.
func (s *Session) Abort() {
	s.stateMu.Lock()
	cancel := s.cancel
	s.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func eventJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `{}`
	}
	return string(data)
}

func sendEvent(stream Stream, ev core.AgentEvent) {
	switch e := ev.(type) {
	case core.EvTextDelta:
		stream.OnText(e.Delta)
	case core.EvToolCall:
		stream.OnEvent("tool_call", eventJSON(map[string]any{"id": e.ID, "name": e.Name, "args": json.RawMessage(e.Args)}))
	case core.EvToolProgress:
		stream.OnEvent("tool_progress", eventJSON(map[string]any{"id": e.ID, "text": e.Text}))
	case core.EvToolResult:
		stream.OnEvent("tool_result", eventJSON(map[string]any{"id": e.ID, "is_error": e.Result.IsError}))
	case core.EvTurnStart:
		stream.OnEvent("turn_start", eventJSON(map[string]any{"step": e.Step}))
	case core.EvTurnEnd:
		if e.Err != nil {
			stream.OnError(e.Err.Error())
		}
		stream.OnEvent("turn_end", eventJSON(map[string]any{"stop": string(e.Stop)}))
	case core.EvUsage:
		stream.OnEvent("usage", eventJSON(map[string]any{"input_tokens": e.Usage.InputTokens, "output_tokens": e.Usage.OutputTokens, "cost_usd": e.Usage.CostUSD}))
	case core.EvError:
		stream.OnError(e.Err.Error())
	}
}

func newAPIKeyClient(name, apiKey string) (provider.Client, string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "anthropic":
		return provider.NewAnthropic(apiKey, ""), "claude-sonnet-4-5", nil
	case "openai":
		return provider.NewOpenAI(apiKey, ""), "gpt-5", nil
	case "openai-responses":
		return provider.NewOpenAIResponses(apiKey, ""), "gpt-5", nil
	case "gemini", "google":
		return provider.NewGemini(apiKey, ""), "gemini-2.5-pro", nil
	case "openai-codex", "codex", "chatgpt":
		return nil, "", fmt.Errorf("%s uses subscription OAuth; use NewSessionWithOAuth", name)
	default:
		return nil, "", fmt.Errorf("unsupported provider: %s", name)
	}
}

func newOAuthClient(name, accessToken, accountID string) (provider.Client, string, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, "", fmt.Errorf("missing OAuth access token")
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "anthropic", "claude":
		return provider.NewAnthropicOAuth(accessToken, ""), "claude-sonnet-4-5", nil
	case "openai", "openai-codex", "codex", "chatgpt":
		if strings.TrimSpace(accountID) == "" {
			return nil, "", fmt.Errorf("missing ChatGPT account id for openai-codex")
		}
		return provider.NewOpenAICodex(accessToken, accountID, ""), "gpt-5.5", nil
	default:
		return nil, "", fmt.Errorf("unsupported OAuth provider: %s", name)
	}
}
