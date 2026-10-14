package chat

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
)

var ErrHubStopped = errors.New("chat hub is stopped")

type onlineRequest struct {
	response chan map[int]bool
}

type botPresenceRequest struct {
	presence string
	response chan struct{}
}

const defaultBotPresence = "Online"

type Hub struct {
	store              Store
	register           chan *Client
	unregister         chan *Client
	broadcast          chan queuedMessage
	online             chan onlineRequest
	botPresence        chan botPresenceRequest
	done               chan struct{}
	botUserID          int
	botInbox           chan BotMessage
	presenceMu         sync.RWMutex
	initialBotPresence string
}

type BotMessage struct {
	UserID  int
	Message IncomingMessage
}

type queuedMessage struct {
	message  IncomingMessage
	senderID int
}

func NewHub(store Store) *Hub {
	return &Hub{
		store:              store,
		register:           make(chan *Client),
		unregister:         make(chan *Client),
		broadcast:          make(chan queuedMessage),
		online:             make(chan onlineRequest),
		botPresence:        make(chan botPresenceRequest),
		done:               make(chan struct{}),
		botInbox:           make(chan BotMessage, 32),
		initialBotPresence: defaultBotPresence,
	}
}

func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)
	clients := make(map[int]*Client)
	for {
		select {
		case <-ctx.Done():
			for _, client := range clients {
				close(client.send)
			}
			return
		case client := <-h.register:
			if old := clients[client.userID]; old != nil {
				close(old.send)
				_ = old.conn.Close()
			}
			clients[client.userID] = client
			h.notifyPresence(ctx, clients, client.userID, true)
		case client := <-h.unregister:
			if clients[client.userID] == client {
				delete(clients, client.userID)
				close(client.send)
				h.notifyPresence(ctx, clients, client.userID, false)
			}
		case request := <-h.online:
			status := make(map[int]bool, len(clients))
			for id := range clients {
				status[id] = true
			}
			request.response <- status
		case request := <-h.botPresence:
			h.presenceMu.Lock()
			h.initialBotPresence = request.presence
			h.presenceMu.Unlock()
			h.notifyBotPresence(ctx, clients, request.presence)
			close(request.response)
		case queued := <-h.broadcast:
			h.route(ctx, clients, queued)
		}
	}
}

func (h *Hub) Register(client *Client) error {
	select {
	case h.register <- client:
		return nil
	case <-h.done:
		return ErrHubStopped
	}
}

func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.done:
	}
}

func (h *Hub) Publish(message IncomingMessage, senderID int) error {
	select {
	case h.broadcast <- queuedMessage{message: message, senderID: senderID}:
		return nil
	case <-h.done:
		return ErrHubStopped
	}
}

func (h *Hub) BotInbox() <-chan BotMessage {
	return h.botInbox
}

func (h *Hub) SetBotUserID(userID int) {
	h.botUserID = userID
}

func (h *Hub) BotUserID() int {
	return h.botUserID
}

func (h *Hub) OnlineUsers() map[int]bool {
	response := make(chan map[int]bool, 1)
	select {
	case h.online <- onlineRequest{response: response}:
		status := <-response
		if h.botUserID > 0 {
			status[h.botUserID] = true
		}
		return status
	case <-h.done:
		return map[int]bool{}
	}
}

func (h *Hub) SetBotPresence(presence string) {
	response := make(chan struct{})
	select {
	case h.botPresence <- botPresenceRequest{presence: presence, response: response}:
		select {
		case <-response:
		case <-h.done:
		}
	case <-h.done:
	}
}

func (h *Hub) BotPresence() string {
	h.presenceMu.RLock()
	defer h.presenceMu.RUnlock()
	return h.initialBotPresence
}

func (h *Hub) notifyBotPresence(ctx context.Context, clients map[int]*Client, presence string) {
	if h.botUserID <= 0 {
		return
	}
	buddies, err := h.store.BuddyIDs(ctx, h.botUserID)
	if err != nil {
		log.Printf("list bot buddies for presence update: %v", err)
		return
	}
	online := true
	event := Event{
		Type:     "presence",
		UserID:   h.botUserID,
		Online:   &online,
		Presence: presence,
	}
	for _, buddyID := range buddies {
		if buddy := clients[buddyID]; buddy != nil && !buddy.deliver(event) {
			delete(clients, buddyID)
			close(buddy.send)
		}
	}
}

func (h *Hub) notifyPresence(ctx context.Context, clients map[int]*Client, userID int, online bool) {
	buddies, err := h.store.BuddyIDs(ctx, userID)
	if err != nil {
		return
	}
	status := online
	for _, buddyID := range buddies {
		event := Event{Type: "presence", UserID: userID, Online: &status}
		if userID == h.botUserID {
			h.presenceMu.RLock()
			event.Presence = h.initialBotPresence
			h.presenceMu.RUnlock()
		}
		if buddy := clients[buddyID]; buddy != nil && !buddy.deliver(event) {
			delete(clients, buddyID)
			close(buddy.send)
		}
	}
}

func (h *Hub) route(ctx context.Context, clients map[int]*Client, queued queuedMessage) {
	if queued.message.Type == "bot_stream" {
		h.routeBotStream(ctx, clients, queued)
		return
	}
	if queued.message.Type == "buzz" {
		h.routeBuzz(ctx, clients, queued)
		return
	}
	if queued.message.Type == "typing" {
		h.routeTyping(ctx, clients, queued)
		return
	}

	message := queued.message
	message.Content = strings.TrimSpace(message.Content)
	if len(message.Content) == 0 || len(message.Content) > 4000 {
		h.sendError(clients[queued.senderID], "message content must be 1-4000 characters")
		return
	}

	var recipients []int
	switch message.Type {
	case "dm":
		if message.RecipientID <= 0 || message.RoomID != 0 || message.RecipientID == queued.senderID {
			h.sendError(clients[queued.senderID], "a direct message requires a different recipient_id")
			return
		}
		allowed, err := h.store.AreBuddies(ctx, queued.senderID, message.RecipientID)
		if err != nil || !allowed {
			h.sendError(clients[queued.senderID], "recipient is not in your buddy list")
			return
		}
		messageRecord := Message{SenderID: queued.senderID, RecipientID: &message.RecipientID, Content: message.Content}
		if err := h.store.SaveMessage(ctx, &messageRecord); err != nil {
			h.sendError(clients[queued.senderID], "could not save message")
			return
		}
		event := Event{Type: "message", Message: &messageRecord}
		if message.RecipientID == h.botUserID {
			select {
			case h.botInbox <- BotMessage{UserID: queued.senderID, Message: message}:
			default:
				h.sendError(clients[queued.senderID], "CyberKid00 is brb lol")
			}
			h.deliverTo(ctx, clients, queued.senderID, event)
			h.sendAck(ctx, clients, queued.senderID, messageRecord.ID, false)
			return
		}
		delivered := h.deliverTo(ctx, clients, *messageRecord.RecipientID, event)
		h.deliverTo(ctx, clients, queued.senderID, event)
		h.sendAck(ctx, clients, queued.senderID, messageRecord.ID, delivered)
	case "room":
		if message.RoomID <= 0 || message.RecipientID != 0 {
			h.sendError(clients[queued.senderID], "a room message requires a room_id")
			return
		}
		member, err := h.store.IsRoomMember(ctx, message.RoomID, queued.senderID)
		if err != nil || !member {
			h.sendError(clients[queued.senderID], "you are not a member of this room")
			return
		}
		recipients, err = h.store.RoomMemberIDs(ctx, message.RoomID)
		if err != nil {
			h.sendError(clients[queued.senderID], "could not find room members")
			return
		}
		messageRecord := Message{SenderID: queued.senderID, RoomID: &message.RoomID, Content: message.Content}
		if err := h.store.SaveMessage(ctx, &messageRecord); err != nil {
			h.sendError(clients[queued.senderID], "could not save message")
			return
		}
		event := Event{Type: "message", Message: &messageRecord}
		delivered := false
		for _, recipientID := range recipients {
			enqueued := h.deliverTo(ctx, clients, recipientID, event)
			if recipientID != queued.senderID && enqueued {
				delivered = true
			}
		}
		h.sendAck(ctx, clients, queued.senderID, messageRecord.ID, delivered)
	default:
		h.sendError(clients[queued.senderID], "type must be dm or room")
	}
}

func (h *Hub) routeBotStream(ctx context.Context, clients map[int]*Client, queued queuedMessage) {
	message := queued.message
	if queued.senderID != h.botUserID || h.botUserID <= 0 ||
		message.RecipientID <= 0 || message.RecipientID == h.botUserID ||
		message.RoomID != 0 || len(message.StreamID) == 0 ||
		len(message.StreamID) > 100 || strings.TrimSpace(message.StreamID) != message.StreamID ||
		(!message.Done && (message.Content == "" || len(message.Content) > 4000)) ||
		(message.Done && message.Content != "") {
		log.Printf("rejected invalid bot stream event from user %d", queued.senderID)
		return
	}
	allowed, err := h.store.AreBuddies(ctx, message.RecipientID, h.botUserID)
	if err != nil {
		log.Printf("validate bot stream recipient %d: %v", message.RecipientID, err)
		return
	}
	if !allowed {
		log.Printf("rejected bot stream to non-buddy user %d", message.RecipientID)
		return
	}
	event := Event{
		Type:        "bot_stream",
		UserID:      h.botUserID,
		RecipientID: message.RecipientID,
		StreamID:    message.StreamID,
		Content:     message.Content,
		Done:        message.Done,
	}
	h.deliverTo(ctx, clients, message.RecipientID, event)
}

func (h *Hub) routeBuzz(ctx context.Context, clients map[int]*Client, queued queuedMessage) {
	message := queued.message
	if message.RecipientID <= 0 || message.RoomID != 0 || message.Content != "" || message.RecipientID == queued.senderID {
		h.sendError(clients[queued.senderID], "a buzz requires a different recipient_id")
		return
	}
	allowed, err := h.store.AreBuddies(ctx, queued.senderID, message.RecipientID)
	if err != nil || !allowed {
		h.sendError(clients[queued.senderID], "recipient is not in your buddy list")
		return
	}

	event := Event{Type: "buzz", UserID: queued.senderID}
	if !h.deliverTo(ctx, clients, message.RecipientID, event) {
		h.sendError(clients[queued.senderID], "buddy is offline; buzz was not delivered")
	}
}

// Typing events are not stored, and invalid ones are dropped without an error to the sender.
func (h *Hub) routeTyping(ctx context.Context, clients map[int]*Client, queued queuedMessage) {
	message := queued.message
	event := Event{Type: "typing", UserID: queued.senderID}

	switch {
	case message.RoomID > 0 && message.RecipientID == 0:
		member, err := h.store.IsRoomMember(ctx, message.RoomID, queued.senderID)
		if err != nil || !member {
			return
		}
		memberIDs, err := h.store.RoomMemberIDs(ctx, message.RoomID)
		if err != nil {
			return
		}
		event.RoomID = message.RoomID
		for _, id := range memberIDs {
			if id != queued.senderID {
				h.deliverTyping(clients, id, event)
			}
		}
	case message.RecipientID > 0 && message.RoomID == 0 && message.RecipientID != queued.senderID:
		allowed, err := h.store.AreBuddies(ctx, queued.senderID, message.RecipientID)
		if err != nil || !allowed {
			return
		}
		h.deliverTyping(clients, message.RecipientID, event)
	}
}

// A full send buffer drops the typing event instead of evicting the client.
func (h *Hub) deliverTyping(clients map[int]*Client, userID int, event Event) {
	if client := clients[userID]; client != nil {
		client.deliver(event)
	}
}

func (h *Hub) deliverTo(ctx context.Context, clients map[int]*Client, userID int, event Event) bool {
	client := clients[userID]
	if client == nil {
		return false
	}
	if !client.deliver(event) {
		delete(clients, userID)
		close(client.send)
		h.notifyPresence(ctx, clients, userID, false)
		return false
	}
	return true
}

func (h *Hub) sendAck(ctx context.Context, clients map[int]*Client, userID int, messageID int64, delivered bool) {
	status := "sent"
	if delivered {
		status = "delivered"
	}
	h.deliverTo(ctx, clients, userID, Event{Type: "ack", MessageID: messageID, Status: status})
}

func (h *Hub) sendError(client *Client, message string) {
	if client != nil {
		client.deliver(Event{Type: "error", Error: message})
	}
}
