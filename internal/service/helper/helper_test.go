package helper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/olelishna/go-shop-diploma/internal/model"
)

func TestSendJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	message := "something went wrong"
	status := http.StatusBadRequest

	SendJSONError(rec, message, status)

	if rec.Code != status {
		t.Errorf("Expected status %d, got %d", status, rec.Code)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", ct)
	}

	var resp model.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error != message {
		t.Errorf("Expected error %q, got %q", message, resp.Error)
	}
}

func TestLuhnValid(t *testing.T) {
	tests := []struct {
		name     string
		number   string
		expected bool
	}{
		// Валидные номера
		{"valid Luhn 79927398713", "79927398713", true},
		{"valid Luhn 4532015112830366", "4532015112830366", true},
		{"valid Luhn 6011111111111117", "6011111111111117", true},
		{"valid zeros 16 digits", "0000000000000000", true},

		// Невалидные
		{"invalid checksum", "79927398714", false},
		{"empty string", "", false}, // ← исправлено!
		{"non-digit chars", "123a45", false},
		{"negative", "-123", false},
		{"spaces not trimmed", " 79927398713", false},
		{"only spaces", "   ", false},
		{"mixed unicode", "7٩٩٢٧٣٩٨٧١٣", false},
		{"newline", "\n79927398713", false}, // тоже нецифра
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LuhnValid(tt.number)
			if got != tt.expected {
				t.Errorf("LuhnValid(%q) = %v, want %v", tt.number, got, tt.expected)
			}
		})
	}
}

func TestIsValidOrderNumber(t *testing.T) {
	tests := []struct {
		name     string
		number   string
		expected bool
	}{
		{"valid Luhn", "79927398713", true},
		{"valid Luhn with spaces", " 79927398713 ", true},
		{"valid Luhn with tabs", "\t79927398713\n", true},
		{"invalid Luhn", "79927398714", false},
		{"empty", "", false},
		{"only spaces", "   ", false},
		{"non-digit", "123abc", false},
		{"negative", "-123", false},
		{"zeros", "0000000000000000", true},
		{"short", "123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidOrderNumber(tt.number)
			if got != tt.expected {
				t.Errorf("IsValidOrderNumber(%q) = %v, want %v", tt.number, got, tt.expected)
			}
		})
	}
}

func TestIsValidOrderNumber_EdgeCases(t *testing.T) {
	cases := []struct {
		desc  string
		input string
		want  bool
	}{
		{"trailing spaces", "123   ", false},
		{"leading zeros (valid)", "0000000000000000", true},
		{"mixed non-digits", "7992a398713", false},
		{"emoji", "7️⃣9️⃣9️⃣2️⃣7️⃣3️⃣9️⃣8️⃣7️⃣1️⃣3️⃣", false},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := IsValidOrderNumber(tc.input)
			if got != tc.want {
				t.Errorf("IsValidOrderNumber(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestSendJSONError_StatusCodes(t *testing.T) {
	codes := map[string]int{
		"Bad Request":           http.StatusBadRequest,
		"Unauthorized":          http.StatusUnauthorized,
		"Internal Server Error": http.StatusInternalServerError,
	}

	for name, code := range codes {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			SendJSONError(rec, "test", code)

			if rec.Code != code {
				t.Errorf("Status mismatch for %s: got %d, want %d", name, rec.Code, code)
			}
		})
	}
}
