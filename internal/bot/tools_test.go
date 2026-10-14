package bot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTriviaToolKeepsPerUserState(t *testing.T) {
	tools := NewToolBox(nil)
	tools.now = func() time.Time { return time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC) }
	ctx := context.Background()

	first, err := tools.Execute(ctx, 11, "let's play", toolStartTrivia, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("start trivia: %v", err)
	}
	if first.(map[string]string)["question"] == "" {
		t.Fatal("start trivia returned an empty question")
	}
	if _, err := tools.Execute(ctx, 12, "anything", toolAnswerTrivia, json.RawMessage(`{}`)); err == nil {
		t.Fatal("answer from a different user unexpectedly accessed the active game")
	}

	if _, err := tools.Execute(ctx, 11, "wrong", toolAnswerTrivia, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("check incorrect answer: %v", err)
	}
	tools.mu.Lock()
	answer := tools.sessions[11].question.answers[0]
	tools.mu.Unlock()
	result, err := tools.Execute(ctx, 11, "I think the answer is "+answer+"!", toolAnswerTrivia, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("check correct answer: %v", err)
	}
	if !result.(map[string]any)["correct"].(bool) {
		t.Fatalf("expected correct answer result, got %#v", result)
	}
	if _, err := tools.Execute(ctx, 11, "again", toolAnswerTrivia, json.RawMessage(`{}`)); err == nil {
		t.Fatal("answered trivia session was not cleared")
	}
}

func TestTriviaToolExpiresSessions(t *testing.T) {
	now := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	tools := NewToolBox(nil)
	tools.now = func() time.Time { return now }
	if _, err := tools.Execute(context.Background(), 7, "start a game", toolStartTrivia, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("start trivia: %v", err)
	}
	now = now.Add(triviaLifetime)
	if _, err := tools.Execute(context.Background(), 7, "anything", toolAnswerTrivia, json.RawMessage(`{}`)); err == nil {
		t.Fatal("expired trivia question remained active")
	}
}

func TestToolArgumentsAreStrictlyValidated(t *testing.T) {
	tools := NewToolBox(nil)
	tests := []struct {
		name        string
		call        string
		userMessage string
		args        string
	}{
		{name: "unknown tool", call: "run_command", args: `{}`},
		{name: "unknown argument", call: toolGetWeather, args: `{"location":"Tokyo","url":"http://example.com"}`},
		{name: "malformed JSON", call: toolAnswerTrivia, args: `{"answer":`},
		{name: "extra JSON value", call: toolStartTrivia, args: `{} {}`},
		{name: "control character", call: toolGetWeather, userMessage: "Tokyo\n", args: `{"location":"Tokyo\n"}`},
		{name: "location too long", call: toolGetWeather, userMessage: strings.Repeat("a", maxWeatherLocationLength+1), args: `{"location":"` + strings.Repeat("a", maxWeatherLocationLength+1) + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := tools.Execute(context.Background(), 1, test.userMessage, test.call, json.RawMessage(test.args)); err == nil {
				t.Fatal("invalid tool call unexpectedly succeeded")
			}
		})
	}
}

func TestDecodeToolArgumentsRejectsUnknownFields(t *testing.T) {
	var args struct {
		Answer string `json:"answer"`
	}
	if err := decodeToolArguments(json.RawMessage(`{"answer":"blue","extra":true}`), &args); err == nil ||
		!strings.Contains(err.Error(), "invalid tool arguments") {
		t.Fatalf("decodeToolArguments() error = %v, want unknown-field error", err)
	}
}

func TestWeatherToolRequiresLocationInLatestUserMessage(t *testing.T) {
	tools := NewToolBox(nil)
	_, err := tools.Execute(context.Background(), 1, "what is the weather?", toolGetWeather, json.RawMessage(`{"location":"Tokyo"}`))
	if err == nil || !strings.Contains(err.Error(), "named in the user's message") {
		t.Fatalf("weather tool error = %v, want missing-location validation", err)
	}
}
