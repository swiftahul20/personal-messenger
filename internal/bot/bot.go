package bot

import (
	"context"
	"log"
	"strings"

	"github.com/swiftahul20/personal-messenger/internal/chat"
)

type Responder interface {
	Reply(context.Context, []chat.Message) (string, error)
}

type StaticResponder struct{}

func (StaticResponder) Reply(_ context.Context, history []chat.Message) (string, error) {
	if len(history) == 0 {
		return "heyyy :) whats up?? lol", nil
	}
	last := strings.TrimSpace(history[len(history)-1].Content)
	if last == "" {
		return "heyyy :) u there? lol", nil
	}
	if strings.Contains(strings.ToLower(last), "help") {
		return "lol /help is on the way :P", nil
	}
	return "heyyy :) im here lol ~*~ wat u up 2?", nil
}

type Bot struct {
	hub       *chat.Hub
	store     chat.Store
	responder Responder
	limiter   *Limiter
	enabled   bool
	botUserID int
}

func NewBot(hub *chat.Hub, store chat.Store, responder Responder, enabled bool, botUserID int, rpm, dailyCap int) *Bot {
	if responder == nil {
		responder = StaticResponder{}
	}
	return &Bot{
		hub:       hub,
		store:     store,
		responder: responder,
		enabled:   enabled,
		botUserID: botUserID,
		limiter:   NewLimiter(rpm, dailyCap),
	}
}

func (b *Bot) Run(ctx context.Context) {
	if !b.enabled {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-b.hub.BotInbox():
			if !ok {
				return
			}
			b.handleMessage(ctx, packet)
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, packet chat.BotMessage) {
	if !b.enabled {
		b.sendAwayMessage(ctx, packet.UserID, "~*~ brb doin bot stuff ~*~")
		return
	}
	if !b.limiter.Allow(packet.UserID) {
		b.sendAwayMessage(ctx, packet.UserID, "~*~ brb eating pizza ~*~")
		return
	}

	history, err := b.store.DMHistory(ctx, packet.UserID, b.botUserID, 0, 20)
	if err != nil {
		log.Printf("bot chat history for user %d: %v", packet.UserID, err)
		history = nil
	}

	if err := b.hub.Publish(chat.IncomingMessage{Type: "typing", RecipientID: packet.UserID}, b.botUserID); err != nil {
		log.Printf("bot typing event for user %d: %v", packet.UserID, err)
		return
	}

	reply, err := b.responder.Reply(ctx, history)
	if err != nil || strings.TrimSpace(reply) == "" {
		reply = "~*~ brb eating pizza ~*~"
	}
	if err := b.hub.Publish(chat.IncomingMessage{Type: "dm", RecipientID: packet.UserID, Content: reply}, b.botUserID); err != nil {
		log.Printf("bot reply to user %d failed: %v", packet.UserID, err)
	}
}

func (b *Bot) sendAwayMessage(ctx context.Context, userID int, message string) {
	_ = ctx
	if err := b.hub.Publish(chat.IncomingMessage{Type: "dm", RecipientID: userID, Content: message}, b.botUserID); err != nil {
		log.Printf("bot away message to user %d failed: %v", userID, err)
	}
}
