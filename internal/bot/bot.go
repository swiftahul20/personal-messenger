package bot

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/swiftahul20/personal-messenger/internal/chat"
)

type Responder interface {
	Reply(context.Context, []chat.Message) (string, error)
}

const (
	PresenceOnline = "Online"
	PresenceAFK    = "AFK"
)

var ErrResponderUnavailable = errors.New("bot responder is unavailable")

type unavailableResponder struct{}

func (unavailableResponder) Reply(context.Context, []chat.Message) (string, error) {
	return "", ErrResponderUnavailable
}

func (unavailableResponder) Available() bool {
	return false
}

type Bot struct {
	hub       *chat.Hub
	store     chat.Store
	responder Responder
	limiter   *Limiter
	tools     *ToolBox
	enabled   bool
	botUserID int
}

func NewBot(hub *chat.Hub, store chat.Store, responder Responder, enabled bool, botUserID int, rpm, dailyCap int) *Bot {
	if responder == nil {
		responder = unavailableResponder{}
	}
	return &Bot{
		hub:       hub,
		store:     store,
		responder: responder,
		enabled:   enabled,
		botUserID: botUserID,
		limiter:   NewLimiter(rpm, dailyCap),
		tools:     NewToolBox(nil),
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
	if reply, handled := commandReply(packet.Message.Content, time.Now()); handled {
		if err := b.hub.Publish(chat.IncomingMessage{
			Type:        "dm",
			RecipientID: packet.UserID,
			Content:     reply,
		}, b.botUserID); err != nil {
			log.Printf("bot command reply to user %d failed: %v", packet.UserID, err)
		}
		return
	}
	if availability, ok := b.responder.(interface{ Available() bool }); ok && !availability.Available() {
		b.hub.SetBotPresence(PresenceAFK)
		return
	}
	if !b.limiter.Allow(packet.UserID) {
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

	var reply string
	if agentic, ok := b.responder.(ToolCallingResponder); ok {
		reply, err = agentic.ReplyWithTools(ctx, history, packet.UserID, packet.Message.Content, b)
	} else {
		reply, err = b.responder.Reply(ctx, history)
	}
	if err != nil {
		log.Printf("bot response to user %d failed: %v", packet.UserID, err)
		b.hub.SetBotPresence(PresenceAFK)
		return
	}
	if strings.TrimSpace(reply) == "" {
		log.Printf("bot response to user %d was empty", packet.UserID)
		b.hub.SetBotPresence(PresenceAFK)
		return
	}
	b.hub.SetBotPresence(PresenceOnline)
	if err := b.hub.Publish(chat.IncomingMessage{Type: "dm", RecipientID: packet.UserID, Content: reply}, b.botUserID); err != nil {
		log.Printf("bot reply to user %d failed: %v", packet.UserID, err)
	}
}

func (b *Bot) Execute(ctx context.Context, userID int, userMessage, name string, arguments json.RawMessage) (any, error) {
	result, err := b.tools.Execute(ctx, userID, userMessage, name, arguments)
	if err != nil {
		log.Printf("bot tool %s for user %d failed: %v", name, userID, err)
	}
	return result, err
}
