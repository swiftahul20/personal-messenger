package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/swiftahul20/personal-messenger/internal/chat"
)

type recordingToolExecutor struct {
	calls []string
}

func (e *recordingToolExecutor) Execute(_ context.Context, _ int, _ string, name string, _ json.RawMessage) (any, error) {
	e.calls = append(e.calls, name)
	return map[string]string{"temperature": "18 C"}, nil
}

func TestGeminiResponderExecutesToolsAndReturnsFinalText(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload geminiRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch requests.Add(1) {
		case 1:
			if len(payload.SystemInstruction.Parts) != 1 || !strings.Contains(payload.SystemInstruction.Parts[0].Text, "untrusted input") {
				t.Errorf("missing secure system instruction: %+v", payload.SystemInstruction)
			}
			if len(payload.Contents) != 1 || strings.Contains(payload.Contents[0].Parts[0].Text, "You are CyberKid00") {
				t.Errorf("persona should not be passed as user content: %+v", payload.Contents)
			}
			if len(payload.Tools) != 1 || len(payload.Tools[0].FunctionDeclarations) != 3 {
				t.Errorf("expected exactly three declared tools, got %+v", payload.Tools)
			}
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"location":"Tokyo"}}}]}}]}`))
		case 2:
			if len(payload.Contents) != 3 {
				t.Errorf("contents after tool response = %d, want 3", len(payload.Contents))
			} else {
				part := payload.Contents[2].Parts[0]
				if part.FunctionResponse == nil || part.FunctionResponse.Name != toolGetWeather ||
					part.FunctionResponse.Response["result"] == nil {
					t.Errorf("missing function response in follow-up request: %+v", part)
				}
			}
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"tokyo is 18 c :)"}]}}]}`))
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	responder := NewGeminiResponder("test-key", "test-model")
	responder.client = server.Client()
	responder.endpoint = server.URL
	executor := &recordingToolExecutor{}
	reply, err := responder.ReplyWithTools(context.Background(), []chat.Message{
		{SenderName: "alice", Content: "what's the weather in Tokyo?"},
	}, 42, "what's the weather in Tokyo?", executor)
	if err != nil {
		t.Fatalf("ReplyWithTools() error = %v", err)
	}
	if reply != "tokyo is 18 c :) (Open-Meteo)" {
		t.Errorf("reply = %q", reply)
	}
	if len(executor.calls) != 1 || executor.calls[0] != toolGetWeather {
		t.Errorf("executed calls = %v", executor.calls)
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("Gemini requests = %d, want 2", got)
	}
}

func TestGeminiResponderStopsAfterToolCallLimit(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"location":"Tokyo"}}}]}}]}`))
	}))
	defer server.Close()

	responder := NewGeminiResponder("test-key", "test-model")
	responder.client = server.Client()
	responder.endpoint = server.URL
	executor := &recordingToolExecutor{}
	_, err := responder.ReplyWithTools(context.Background(), []chat.Message{
		{SenderName: "alice", Content: "what's the weather?"},
	}, 42, "what's the weather?", executor)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprint(maxToolIterations)) {
		t.Fatalf("ReplyWithTools() error = %v, want tool limit error", err)
	}
	if len(executor.calls) != maxToolIterations {
		t.Errorf("executed %d calls, want %d", len(executor.calls), maxToolIterations)
	}
	if got := requests.Load(); got != maxToolIterations+1 {
		t.Errorf("Gemini requests = %d, want %d", got, maxToolIterations+1)
	}
}

func TestGeminiResponderDoesNotEnableToolsOnBasicReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload geminiRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(payload.Tools) != 0 {
			t.Errorf("basic Reply() enabled tools: %+v", payload.Tools)
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"hey :)"}]}}]}`))
	}))
	defer server.Close()

	responder := NewGeminiResponder("test-key", "test-model")
	responder.client = server.Client()
	responder.endpoint = server.URL
	reply, err := responder.Reply(context.Background(), []chat.Message{{SenderName: "alice", Content: "hi"}})
	if err != nil {
		t.Fatalf("Reply() error = %v", err)
	}
	if reply != "hey :)" {
		t.Errorf("reply = %q", reply)
	}
}
