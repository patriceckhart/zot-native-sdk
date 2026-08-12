package zot

import (
	"strings"
	"testing"
)

func TestCredentialValidation(t *testing.T) {
	if _, err := NewSession("anthropic", "", "", ""); err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected missing API key error, got %v", err)
	}
	if _, err := NewSessionWithOAuth("anthropic", "", "", "", ""); err == nil || !strings.Contains(err.Error(), "OAuth") {
		t.Fatalf("expected missing OAuth token error, got %v", err)
	}
	if _, err := NewSessionWithOAuth("openai-codex", "token", "", "", ""); err == nil || !strings.Contains(err.Error(), "account id") {
		t.Fatalf("expected missing account id error, got %v", err)
	}
}

func TestHistoryRoundTrip(t *testing.T) {
	session, err := NewSession("anthropic", "synthetic-key", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := session.ExportHistory(); got != "[]" {
		t.Fatalf("expected empty history, got %s", got)
	}
	if err := session.ImportHistory("[]"); err != nil {
		t.Fatalf("import empty history: %v", err)
	}
	if err := session.ImportHistory("not-json"); err == nil {
		t.Fatal("expected malformed history error")
	}
	if err := session.ImportHistory("null"); err == nil {
		t.Fatal("expected non-array history error")
	}
	if err := session.ImportHistory(strings.Repeat(" ", 16*1024*1024+1)); err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("expected history size error, got %v", err)
	}
}

func TestUnsupportedProvider(t *testing.T) {
	if _, err := NewSession("unknown", "secret", "", ""); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported provider error, got %v", err)
	}
}
