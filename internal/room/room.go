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

type Store interface {
	CreateRoom(context.Context, int, string) (Room, error)
	JoinRoom(context.Context, int, int) error
	ListRooms(context.Context, int) ([]Room, error)
}

func ValidateName(name string) error {
	if len([]rune(name)) == 0 || len([]rune(name)) > 100 {
		return ErrInvalidName
	}
	return nil
}
