package chat

import (
	"context"
	"time"
)

type Message struct {
	ID          int64     `json:"id"`
	SenderID    int       `json:"sender_id"`
	RecipientID *int      `json:"recipient_id,omitempty"`
	RoomID      *int      `json:"room_id,omitempty"`
	Content     string    `json:"content"`
	SentAt      time.Time `json:"sent_at"`
}

type IncomingMessage struct {
	Type        string `json:"type"`
	RecipientID int    `json:"recipient_id,omitempty"`
	RoomID      int    `json:"room_id,omitempty"`
	Content     string `json:"content"`
}

type Event struct {
	Type    string   `json:"type"`
	Message *Message `json:"message,omitempty"`
	UserID  int      `json:"user_id,omitempty"`
	RoomID  int      `json:"room_id,omitempty"`
	Online  *bool    `json:"online,omitempty"`
	Error   string   `json:"error,omitempty"`
}

type Store interface {
	SaveMessage(context.Context, *Message) error
	AreBuddies(context.Context, int, int) (bool, error)
	IsRoomMember(context.Context, int, int) (bool, error)
	RoomMemberIDs(context.Context, int) ([]int, error)
	BuddyIDs(context.Context, int) ([]int, error)
	DMHistory(context.Context, int, int, int) ([]Message, error)
	RoomHistory(context.Context, int, int) ([]Message, error)
}
