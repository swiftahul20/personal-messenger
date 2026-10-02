package chat

import (
	"context"
	"errors"
	"strings"
)

var ErrHubStopped = errors.New("chat hub is stopped")

type onlineRequest struct {
	response chan map[int]bool
}

type Hub struct {
	store      Store
	register   chan *Client
	unregister chan *Client
	broadcast  chan queuedMessage
	online     chan onlineRequest
	done       chan struct{}
}

type queuedMessage struct {
	message  IncomingMessage
	senderID int
}

func NewHub(store Store) *Hub {
	return &Hub{
		store:      store,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan queuedMessage),
		online:     make(chan onlineRequest),
		done:       make(chan struct{}),
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

func (h *Hub) OnlineUsers() map[int]bool {
	response := make(chan map[int]bool, 1)
	select {
	case h.online <- onlineRequest{response: response}:
		return <-response
	case <-h.done:
		return map[int]bool{}
	}
}

func (h *Hub) notifyPresence(ctx context.Context, clients map[int]*Client, userID int, online bool) {
	buddies, err := h.store.BuddyIDs(ctx, userID)
	if err != nil {
		return
	}
	status := online
	for _, buddyID := range buddies {
		if buddy := clients[buddyID]; buddy != nil && !buddy.deliver(Event{Type: "presence", UserID: userID, Online: &status}) {
			delete(clients, buddyID)
			close(buddy.send)
		}
	}
}

func (h *Hub) route(ctx context.Context, clients map[int]*Client, queued queuedMessage) {
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
		h.deliverTo(ctx, clients, queued.senderID, event)
		h.deliverTo(ctx, clients, *messageRecord.RecipientID, event)
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
		for _, recipientID := range recipients {
			h.deliverTo(ctx, clients, recipientID, event)
		}
	default:
		h.sendError(clients[queued.senderID], "type must be dm or room")
	}
}

func (h *Hub) deliverTo(ctx context.Context, clients map[int]*Client, userID int, event Event) {
	client := clients[userID]
	if client != nil && !client.deliver(event) {
		delete(clients, userID)
		close(client.send)
		h.notifyPresence(ctx, clients, userID, false)
	}
}

func (h *Hub) sendError(client *Client, message string) {
	if client != nil {
		client.deliver(Event{Type: "error", Error: message})
	}
}
