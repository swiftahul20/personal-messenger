package bot

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/swiftahul20/personal-messenger/internal/chat"
)

func TestCommandReply(t *testing.T) {
	now := time.Date(2026, time.October, 12, 11, 5, 0, 0, time.FixedZone("ICT", 7*60*60))
	tests := []struct {
		name    string
		content string
		want    string
		handled bool
	}{
		{name: "help", content: "/help", want: "/8ball", handled: true},
		{name: "8ball without question", content: "/8ball", want: "question", handled: true},
		{name: "time", content: "/time", want: "11:05 AM ICT", handled: true},
		{name: "roll defaults", content: "/roll", want: `rolled 1d6: [1-6] = [1-6] :D`, handled: true},
		{name: "roll dice notation", content: "/roll 3d4", want: `rolled 3d4: [1-4] \+ [1-4] \+ [1-4] = ([3-9]|1[0-2]) :D`, handled: true},
		{name: "roll invalid notation", content: "/roll 101d6", want: "usage:", handled: true},
		{name: "8ball answer", content: "/8ball will it work?", want: "", handled: true},
		{name: "unknown command", content: "/dance", handled: false},
		{name: "ordinary message", content: "hello", handled: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reply, handled := commandReply(test.content, now)
			if handled != test.handled {
				t.Fatalf("commandReply() handled=%v, want %v", handled, test.handled)
			}
			if !handled {
				return
			}
			if test.want == "" {
				if !containsAnswer(eightBallAnswers[:], reply) {
					t.Fatalf("unexpected 8ball reply %q", reply)
				}
				return
			}
			if test.name == "help" && (!strings.Contains(reply, "trivia") || !strings.Contains(reply, "weather")) {
				t.Errorf("help reply omits new tools: %q", reply)
			}
			if matched, _ := regexp.MatchString(test.want, reply); !matched {
				t.Fatalf("commandReply() = %q, want pattern %q", reply, test.want)
			}
		})
	}
}

func containsAnswer(answers []string, reply string) bool {
	for _, answer := range answers {
		if answer == reply {
			return true
		}
	}
	return false
}

type commandStore struct {
	saved chan chat.Message
}

func (s commandStore) SaveMessage(_ context.Context, message *chat.Message) error {
	message.ID = 1
	message.SentAt = time.Now()
	s.saved <- *message
	return nil
}

func (commandStore) AreBuddies(context.Context, int, int) (bool, error) { return true, nil }

func (commandStore) IsRoomMember(context.Context, int, int) (bool, error) {
	return false, nil
}

func (commandStore) RoomMemberIDs(context.Context, int) ([]int, error) { return nil, nil }
func (commandStore) BuddyIDs(context.Context, int) ([]int, error)      { return nil, nil }
func (commandStore) DMHistory(context.Context, int, int, int64, int) ([]chat.Message, error) {
	return nil, nil
}
func (commandStore) RoomHistory(context.Context, int, int64, int) ([]chat.Message, error) {
	return nil, nil
}

type countingResponder struct {
	calls int
}

func (r *countingResponder) Reply(context.Context, []chat.Message) (string, error) {
	r.calls++
	return "unexpected LLM reply", nil
}

type failingResponder struct{}

func (failingResponder) Reply(context.Context, []chat.Message) (string, error) {
	return "", errors.New("provider unavailable")
}

type fixedResponder string

func (r fixedResponder) Reply(context.Context, []chat.Message) (string, error) {
	return string(r), nil
}

func TestCommandReplyDoesNotUseResponderOrLimiter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := commandStore{saved: make(chan chat.Message, 1)}
	hub := chat.NewHub(store)
	hub.SetBotUserID(2)
	go hub.Run(ctx)

	responder := &countingResponder{}
	bot := NewBot(hub, store, responder, true, 2, 1, 1)
	bot.handleMessage(ctx, chat.BotMessage{
		UserID: 1,
		Message: chat.IncomingMessage{
			Type:    "dm",
			Content: "/help",
		},
	})

	select {
	case message := <-store.saved:
		if message.SenderID != 2 || message.RecipientID == nil || *message.RecipientID != 1 {
			t.Fatalf("command response was routed as %+v", message)
		}
		if message.Content == "" {
			t.Fatal("command response was empty")
		}
	case <-time.After(time.Second):
		t.Fatal("command response was not published")
	}
	if responder.calls != 0 {
		t.Fatalf("LLM responder was called %d times", responder.calls)
	}
	bot.limiter.mu.Lock()
	dailyCalls := bot.limiter.dailyCount
	bot.limiter.mu.Unlock()
	if dailyCalls != 0 {
		t.Fatalf("command used %d LLM quota slots", dailyCalls)
	}
}

func TestMissingResponderMarksBotAFKWithoutSendingFallbackDM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := commandStore{saved: make(chan chat.Message, 1)}
	hub := chat.NewHub(store)
	hub.SetBotUserID(2)
	go hub.Run(ctx)
	hub.SetBotPresence(PresenceOnline)
	bot := NewBot(hub, store, nil, true, 2, 5, 100)

	bot.handleMessage(ctx, chat.BotMessage{
		UserID: 1,
		Message: chat.IncomingMessage{
			Type:    "dm",
			Content: "hello",
		},
	})

	if got := hub.BotPresence(); got != PresenceAFK {
		t.Fatalf("BotPresence() = %q, want %q", got, PresenceAFK)
	}
	select {
	case message := <-store.saved:
		t.Fatalf("missing API generated a hardcoded fallback DM: %+v", message)
	default:
	}
}

func TestResponderFailureMarksBotAFKAndSuccessfulReplyRestoresOnline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := commandStore{saved: make(chan chat.Message, 2)}
	hub := chat.NewHub(store)
	hub.SetBotUserID(2)
	go hub.Run(ctx)
	hub.SetBotPresence(PresenceOnline)
	bot := NewBot(hub, store, failingResponder{}, true, 2, 5, 100)
	packet := chat.BotMessage{
		UserID: 1,
		Message: chat.IncomingMessage{
			Type:    "dm",
			Content: "hello",
		},
	}

	bot.handleMessage(ctx, packet)
	if got := hub.BotPresence(); got != PresenceAFK {
		t.Fatalf("presence after responder failure = %q, want %q", got, PresenceAFK)
	}
	select {
	case message := <-store.saved:
		t.Fatalf("responder error generated an automated fallback DM: %+v", message)
	default:
	}

	bot.responder = fixedResponder("Gemini response")
	bot.handleMessage(ctx, packet)
	if got := hub.BotPresence(); got != PresenceOnline {
		t.Fatalf("presence after successful response = %q, want %q", got, PresenceOnline)
	}
	select {
	case message := <-store.saved:
		if message.Content != "Gemini response" {
			t.Fatalf("reply = %q, want Gemini response", message.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("successful response was not sent")
	}
}
