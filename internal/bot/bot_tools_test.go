package bot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/swiftahul20/personal-messenger/internal/chat"
)

type triviaResponder struct {
	userID int
}

func (triviaResponder) Reply(context.Context, []chat.Message) (string, error) {
	return "", nil
}

func (r *triviaResponder) ReplyWithTools(ctx context.Context, _ []chat.Message, userID int, userMessage string, tools ToolExecutor) (string, error) {
	r.userID = userID
	result, err := tools.Execute(ctx, userID, userMessage, toolStartTrivia, json.RawMessage(`{}`))
	if err != nil {
		return "", err
	}
	return result.(map[string]string)["question"], nil
}

func TestBotConnectsGeminiToolCallsToUserScopedToolbox(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := commandStore{saved: make(chan chat.Message, 1)}
	hub := chat.NewHub(store)
	hub.SetBotUserID(2)
	go hub.Run(ctx)

	responder := &triviaResponder{}
	bot := NewBot(hub, store, responder, true, 2, 5, 100)
	bot.handleMessage(ctx, chat.BotMessage{
		UserID: 41,
		Message: chat.IncomingMessage{
			Type:    "dm",
			Content: "let's play trivia",
		},
	})
	select {
	case message := <-store.saved:
		if message.Content == "" || !strings.Contains(message.Content, "?") {
			t.Fatalf("trivia response = %q, want question", message.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("bot did not publish the trivia question")
	}
	if responder.userID != 41 {
		t.Errorf("tool call user ID = %d, want 41", responder.userID)
	}
}
