package user

import (
	"context"
	"errors"
	"regexp"
	"time"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,50}$`)
var ErrInvalidUsername = errors.New("username must be 3-50 characters using letters, numbers, or underscores")

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type Buddy struct {
	User
	Online bool `json:"online"`
}

type Store interface {
	CreateOrGet(context.Context, string) (User, error)
	FindByUsername(context.Context, string) (User, error)
	GetByID(context.Context, int) (User, error)
	AddBuddy(context.Context, int, int) error
	ListBuddies(context.Context, int) ([]User, error)
}

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return ErrInvalidUsername
	}
	return nil
}
