package user

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

const DefaultBotUsername = "CyberKid00"

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,50}$`)
var ErrInvalidUsername = errors.New("username must be 3-50 characters using letters, numbers, or underscores")
var ErrReservedUsername = errors.New("username is reserved for the bot")

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type Buddy struct {
	User
	Online   bool   `json:"online"`
	Presence string `json:"presence,omitempty"`
}

type Store interface {
	CreateOrGet(context.Context, string) (User, error)
	FindByUsername(context.Context, string) (User, error)
	GetByID(context.Context, int) (User, error)
	AddBuddy(context.Context, int, int) error
	ListBuddies(context.Context, int) ([]User, error)
}

func IsReservedUsername(username string) bool {
	return strings.EqualFold(strings.TrimSpace(username), DefaultBotUsername)
}

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return ErrInvalidUsername
	}
	if IsReservedUsername(username) {
		return ErrReservedUsername
	}
	return nil
}
