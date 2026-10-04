package room

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidName = errors.New("room name must be 1-100 characters")

type Room struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Listing struct {
	Room
	MemberCount int  `json:"member_count"`
	Joined      bool `json:"joined"`
}

type Member struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Online   bool   `json:"online"`
}

type Store interface {
	CreateRoom(context.Context, int, string) (Room, error)
	JoinRoom(context.Context, int, int) error
	ListRooms(context.Context, int) ([]Room, error)
	BrowseRooms(ctx context.Context, userID int, query string) ([]Listing, error)
	RoomMembers(ctx context.Context, roomID int) ([]Member, error)
}

func ValidateName(name string) error {
	if len([]rune(name)) == 0 || len([]rune(name)) > 100 {
		return ErrInvalidName
	}
	return nil
}
