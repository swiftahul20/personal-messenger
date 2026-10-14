package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/swiftahul20/personal-messenger/internal/chat"
)

type triviaResponder struct {
	userID int
}

type streamingResponder struct{}

func (streamingResponder) Reply(context.Context, []chat.Message) (string, error) {
	return "hello there", nil
}

func (streamingResponder) ReplyWithToolsStream(_ context.Context, _ []chat.Message, _ int, _ string, _ ToolExecutor, onToken func(string) error) (string, error) {
	if err := onToken("hello "); err != nil {
		return "", err
	}
	if err := onToken("there"); err != nil {
		return "", err
	}
	return "hello there", nil
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

func TestBotStreamsChunksThenPublishesPersistedReply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := commandStore{saved: make(chan chat.Message, 1)}
	hub := chat.NewHub(store)
	hub.SetBotUserID(2)
	go hub.Run(ctx)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		chat.NewClient(41, hub, conn).Run()
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline := time.Now().Add(time.Second)
	for !hub.OnlineUsers()[41] && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !hub.OnlineUsers()[41] {
		t.Fatal("websocket client did not register with the hub")
	}

	bot := NewBot(hub, store, streamingResponder{}, true, 2, 5, 100)
	bot.handleMessage(ctx, chat.BotMessage{
		UserID: 41,
		Message: chat.IncomingMessage{
			Type:    "dm",
			Content: "hello",
		},
	})

	var events []chat.Event
	for len(events) < 5 {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		var event chat.Event
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatalf("received %d events, want typing, two chunks, final message, and completion: %v", len(events), err)
		}
		events = append(events, event)
	}
	if events[0].Type != "typing" {
		t.Errorf("first event = %q, want typing", events[0].Type)
	}
	if events[1].Type != "bot_stream" || events[1].Content != "hello " ||
		events[2].Type != "bot_stream" || events[2].Content != "there" {
		t.Errorf("stream events = %+v, %+v", events[1], events[2])
	}
	if events[3].Type != "message" || events[3].Message == nil ||
		events[3].Message.Content != "hello there" {
		t.Errorf("final persisted message = %+v", events[3])
	}
	if events[4].Type != "bot_stream" || !events[4].Done {
		t.Errorf("stream completion = %+v", events[4])
	}
	select {
	case saved := <-store.saved:
		if saved.Content != "hello there" {
			t.Errorf("saved reply = %q", saved.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("final bot reply was not persisted")
	}
}
