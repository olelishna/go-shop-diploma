package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUser_Create(t *testing.T) {
	created := time.Now().Truncate(time.Second)
	user := User{
		ID:           123,
		Login:        "testuser",
		PasswordHash: "$2a$10$...",
		CreatedAt:    created,
	}

	if user.ID != 123 {
		t.Errorf("ID mismatch: got %d, want 123", user.ID)
	}

	if user.Login != "testuser" {
		t.Errorf("Login mismatch: got %q, want %q", user.Login, "testuser")
	}

	if user.PasswordHash != "$2a$10$..." {
		t.Errorf("PasswordHash mismatch: got %q", user.PasswordHash)
	}

	if user.CreatedAt.Before(created.Add(-time.Second)) ||
		user.CreatedAt.After(created.Add(time.Second)) {
		t.Errorf("CreatedAt out of range: got %v", user.CreatedAt)
	}
}

func TestUser_ValueSemantics(t *testing.T) {
	user1 := User{
		ID:        456,
		Login:     "userA",
		CreatedAt: time.Date(2025, 4, 5, 12, 0, 0, 0, time.UTC),
	}
	user2 := user1

	user1.ID = 789
	user1.Login = "userB"

	if user2.ID != 456 {
		t.Errorf("user2.ID changed unexpectedly: got %d, want 456", user2.ID)
	}

	if user2.Login != "userA" {
		t.Errorf("user2.Login changed unexpectedly: got %q", user2.Login)
	}
}

func TestUser_JSONMarshal(t *testing.T) {
	user := User{
		ID:           999,
		Login:        "jsonuser",
		PasswordHash: "hash123",
		CreatedAt:    time.Unix(1712345678, 0).UTC(),
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expectedFields := []string{"ID", "Login", "PasswordHash", "CreatedAt"}
	for _, f := range expectedFields {
		if !contains(string(data), f) {
			t.Errorf("JSON is missing field %q: %s", f, string(data))
		}
	}
}

func TestUser_JSONUnmarshal(t *testing.T) {
	input := `{"ID":101,"Login":"unmarshal_user","PasswordHash":"hash456","CreatedAt":"2025-04-05T12:34:56Z"}`

	var user User

	if err := json.Unmarshal([]byte(input), &user); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if user.ID != 101 {
		t.Errorf("ID mismatch: got %d, want 101", user.ID)
	}

	if user.Login != "unmarshal_user" {
		t.Errorf("Login mismatch: got %q, want %q", user.Login, "unmarshal_user")
	}

	if user.PasswordHash != "hash456" {
		t.Errorf("PasswordHash mismatch: got %q", user.PasswordHash)
	}

	if user.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero after unmarshal")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
