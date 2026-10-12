package user

import "testing"

func TestValidateUsernameAcceptsNormalNames(t *testing.T) {
	if err := ValidateUsername("alice_42"); err != nil {
		t.Fatalf("valid username rejected: %v", err)
	}
}

func TestValidateUsernameRejectsReservedBotName(t *testing.T) {
	if err := ValidateUsername(DefaultBotUsername); err == nil {
		t.Fatal("reserved bot username was accepted")
	}
	if err := ValidateUsername("cyberkid00"); err == nil {
		t.Fatal("case-insensitive reserved bot username was accepted")
	}
}
