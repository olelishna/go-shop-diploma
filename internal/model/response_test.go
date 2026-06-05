package model

import (
	"encoding/json"
	"testing"
)

func TestErrorResponse_JSON(t *testing.T) {
	tests := []struct {
		name     string
		response ErrorResponse
		expected string
	}{
		{
			name:     "simple error",
			response: ErrorResponse{Error: "not found"},
			expected: `{"error":"not found"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(data))
			}

			var unmarshaled ErrorResponse

			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if unmarshaled.Error != tt.response.Error {
				t.Errorf(
					"Unmarshaled error doesn't match: expected %q, got %q",
					tt.response.Error,
					unmarshaled.Error,
				)
			}
		})
	}
}

func TestBalanceResponse_JSON(t *testing.T) {
	tests := []struct {
		name     string
		response BalanceResponse
		expected string
	}{
		{
			name: "with positive values",
			response: BalanceResponse{
				Current:   1234.56,
				Withdrawn: 234.56,
			},
			expected: `{"current":1234.56,"withdrawn":234.56}`,
		},
		{
			name: "with zero values",
			response: BalanceResponse{
				Current:   0,
				Withdrawn: 0,
			},
			expected: `{"current":0,"withdrawn":0}`,
		},
		{
			name: "negative withdrawn (possible)",
			response: BalanceResponse{
				Current:   100,
				Withdrawn: -10,
			},
			expected: `{"current":100,"withdrawn":-10}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(data))
			}

			var unmarshaled BalanceResponse
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if unmarshaled.Current != tt.response.Current ||
				unmarshaled.Withdrawn != tt.response.Withdrawn {
				t.Errorf("Fields mismatch: expected (%v,%v), got (%v,%v)",
					tt.response.Current, tt.response.Withdrawn,
					unmarshaled.Current, unmarshaled.Withdrawn)
			}
		})
	}
}

func TestAccrualResponse_JSON(t *testing.T) {
	accrualVal := 45.67

	tests := []struct {
		name     string
		response AccrualResponse
		expected string
	}{
		{
			name: "with accrual present",
			response: AccrualResponse{
				Order:   "ORD123",
				Status:  "PROCESSED",
				Accrual: &accrualVal,
			},
			expected: `{"order":"ORD123","status":"PROCESSED","accrual":45.67}`,
		},
		{
			name: "with accrual nil (omitempty)",
			response: AccrualResponse{
				Order:   "ORD456",
				Status:  "INVALID",
				Accrual: nil,
			},
			expected: `{"order":"ORD456","status":"INVALID"}`,
		},
		{
			name: "with zero accrual",
			response: AccrualResponse{
				Order:   "ORD789",
				Status:  "PROCESSING",
				Accrual: floatPtr(0.0),
			},
			expected: `{"order":"ORD789","status":"PROCESSING","accrual":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(data))
			}

			var unmarshaled AccrualResponse
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if unmarshaled.Order != tt.response.Order || unmarshaled.Status != tt.response.Status {
				t.Errorf("Order/Status mismatch: expected (%s,%s), got (%s,%s)",
					tt.response.Order, tt.response.Status, unmarshaled.Order, unmarshaled.Status)
			}

			if (tt.response.Accrual == nil) != (unmarshaled.Accrual == nil) {
				t.Errorf(
					"Accrual nilness mismatch: expected nil=%v, got %v",
					tt.response.Accrual == nil,
					unmarshaled.Accrual == nil,
				)
			} else if tt.response.Accrual != nil && unmarshaled.Accrual != nil {
				if *tt.response.Accrual != *unmarshaled.Accrual {
					t.Errorf(
						"Accrual value mismatch: expected %v, got %v",
						*tt.response.Accrual,
						*unmarshaled.Accrual,
					)
				}
			}
		})
	}
}

func TestWithdrawalResponse_JSON(t *testing.T) {
	tests := []struct {
		name     string
		response WithdrawalResponse
		expected string
	}{
		{
			name: "full response",
			response: WithdrawalResponse{
				Order:       "ORD123",
				Sum:         150.0,
				ProcessedAt: "2025-04-05T16:30:00Z",
			},
			expected: `{"order":"ORD123","sum":150,"processed_at":"2025-04-05T16:30:00Z"}`,
		},
		{
			name: "with fractional sum",
			response: WithdrawalResponse{
				Order:       "ORD456",
				Sum:         99.99,
				ProcessedAt: "2025-04-05T17:00:00Z",
			},
			expected: `{"order":"ORD456","sum":99.99,"processed_at":"2025-04-05T17:00:00Z"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(data))
			}

			var unmarshaled WithdrawalResponse
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if unmarshaled.Order != tt.response.Order || unmarshaled.Sum != tt.response.Sum ||
				unmarshaled.ProcessedAt != tt.response.ProcessedAt {
				t.Errorf("Fields mismatch")
			}
		})
	}
}
