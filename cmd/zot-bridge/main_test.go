package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bridge "github.com/patriceckhart/zot-native-sdk/bridge"
)

type fakeSession struct {
	mu          sync.Mutex
	history     string
	abortCount  int
	active      atomic.Int32
	maxActive   atomic.Int32
	promptStart chan struct{}
	release     chan struct{}
}

func (f *fakeSession) Prompt(message string, output bridge.Stream) {
	active := f.active.Add(1)
	for {
		maximum := f.maxActive.Load()
		if active <= maximum || f.maxActive.CompareAndSwap(maximum, active) {
			break
		}
	}
	if f.promptStart != nil {
		f.promptStart <- struct{}{}
	}
	if f.release != nil {
		<-f.release
	}
	output.OnEvent("turn_start", `{"step":1}`)
	output.OnText(message)
	f.mu.Lock()
	f.history = `[{"role":"user"}]`
	f.mu.Unlock()
	f.active.Add(-1)
	output.OnDone()
}

func (f *fakeSession) ExportHistory() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.history == "" {
		return "[]"
	}
	return f.history
}

func (f *fakeSession) ImportHistory(history string) error {
	f.mu.Lock()
	f.history = history
	f.mu.Unlock()
	return nil
}

func (f *fakeSession) Abort() {
	f.mu.Lock()
	f.abortCount++
	f.mu.Unlock()
}

func decodeLines(t *testing.T, output string) []map[string]any {
	t.Helper()
	var values []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		values = append(values, value)
	}
	return values
}

func TestPromptEventOrderingAndHistory(t *testing.T) {
	var output bytes.Buffer
	server := newServer(&output)
	fake := &fakeSession{}
	server.newAPIKey = func(_, _, _, _ string) (session, error) { return fake, nil }

	server.handle(request{ID: 1, Method: "create_session", SessionID: "test", Provider: "anthropic", APIKey: "key"})
	server.handle(request{ID: 2, Method: "prompt", SessionID: "test", Message: "hello"})
	server.runWG.Wait()

	lines := decodeLines(t, output.String())
	if len(lines) != 6 {
		t.Fatalf("expected 6 protocol lines, got %d: %s", len(lines), output.String())
	}
	if lines[1]["id"] != float64(2) {
		t.Fatalf("prompt acknowledgement must precede events: %s", output.String())
	}
	want := []string{"turn_start", "text", "history", "done"}
	for index, event := range want {
		if got := lines[index+2]["event"]; got != event {
			t.Fatalf("event %d: got %v, want %s", index, got, event)
		}
	}
	if history := lines[4]["payload"].(map[string]any)["history"]; history != `[{"role":"user"}]` {
		t.Fatalf("unexpected emitted history: %v", history)
	}
}

func TestConcurrentPromptsAreDispatchedWithoutBlockingInput(t *testing.T) {
	var output bytes.Buffer
	server := newServer(&output)
	fake := &fakeSession{promptStart: make(chan struct{}, 2), release: make(chan struct{}, 2)}
	server.newAPIKey = func(_, _, _, _ string) (session, error) { return fake, nil }
	server.handle(request{ID: 1, Method: "create_session", SessionID: "test", APIKey: "key"})

	server.handle(request{ID: 2, Method: "prompt", SessionID: "test", Message: "one"})
	server.handle(request{ID: 3, Method: "prompt", SessionID: "test", Message: "two"})
	select {
	case <-fake.promptStart:
	case <-time.After(time.Second):
		t.Fatal("first prompt did not start")
	}
	fake.release <- struct{}{}
	select {
	case <-fake.promptStart:
	case <-time.After(time.Second):
		t.Fatal("second prompt did not start")
	}
	fake.release <- struct{}{}
	server.runWG.Wait()
	if maximum := fake.maxActive.Load(); maximum != 2 {
		t.Fatalf("expected both acknowledged prompts to run, maximum concurrency was %d", maximum)
	}
}

func TestSessionLifecycleAndErrors(t *testing.T) {
	var output bytes.Buffer
	server := newServer(&output)
	first := &fakeSession{}
	second := &fakeSession{}
	created := 0
	server.newAPIKey = func(_, _, _, _ string) (session, error) {
		created++
		if created == 1 {
			return first, nil
		}
		return second, nil
	}

	server.handle(request{ID: 1, Method: "create_session", APIKey: "key"})
	server.handle(request{ID: 2, Method: "prompt", SessionID: "missing", Message: "hello"})
	server.handle(request{ID: 3, Method: "unknown"})
	server.handle(request{ID: 4, Method: "create_session", SessionID: "same", APIKey: "key"})
	server.handle(request{ID: 5, Method: "create_session", SessionID: "same", APIKey: "key"})
	server.handle(request{ID: 6, Method: "close_session", SessionID: "same"})
	server.handle(request{ID: 7, Method: "close_session", SessionID: "same"})

	first.mu.Lock()
	firstAborts := first.abortCount
	first.mu.Unlock()
	second.mu.Lock()
	secondAborts := second.abortCount
	second.mu.Unlock()
	if firstAborts != 1 || secondAborts != 1 {
		t.Fatalf("replacement and close must abort sessions, got %d and %d", firstAborts, secondAborts)
	}

	lines := decodeLines(t, output.String())
	for _, index := range []int{0, 1, 2} {
		if _, ok := lines[index]["error"]; !ok {
			t.Fatalf("response %d should be an error: %#v", index, lines[index])
		}
	}
	if closed := lines[5]["result"].(map[string]any)["closed"]; closed != true {
		t.Fatalf("first close should report closed: %v", closed)
	}
	if closed := lines[6]["result"].(map[string]any)["closed"]; closed != false {
		t.Fatalf("second close should report not closed: %v", closed)
	}
}

func TestCloseAllAbortsEverySession(t *testing.T) {
	server := newServer(&bytes.Buffer{})
	first := &fakeSession{}
	second := &fakeSession{}
	server.sessions["first"] = first
	server.sessions["second"] = second
	server.closeAll()

	if len(server.sessions) != 0 {
		t.Fatal("closeAll did not clear sessions")
	}
	if first.abortCount != 1 || second.abortCount != 1 {
		t.Fatalf("closeAll did not abort sessions: %d, %d", first.abortCount, second.abortCount)
	}
}
