package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOrderStatusConstants(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{status: StatusNew, want: "NEW"},
		{status: StatusProcessing, want: "PROCESSING"},
		{status: StatusInvalid, want: "INVALID"},
		{status: StatusProcessed, want: "PROCESSED"},
	}

	for _, tt := range tests {
		if tt.status != tt.want {
			t.Errorf("Expected status %q, got %q", tt.want, tt.status)
		}
	}
}

func TestOrderResponse_JSONMarshal(t *testing.T) {
	accrual := 123.45
	tests := []struct {
		name     string
		order    OrderResponse
		expected string
	}{
		{
			name: "with accrual",
			order: OrderResponse{
				Number:     "ORD123",
				Status:     StatusProcessed,
				Accrual:    &accrual,
				UploadedAt: "2025-04-05T12:00:00Z",
			},
			expected: `{"number":"ORD123","status":"PROCESSED","accrual":123.45,"uploaded_at":"2025-04-05T12:00:00Z"}`,
		},
		{
			name: "without accrual (nil)",
			order: OrderResponse{
				Number:     "ORD456",
				Status:     StatusNew,
				Accrual:    nil,
				UploadedAt: "2025-04-05T13:00:00Z",
			},
			expected: `{"number":"ORD456","status":"NEW","uploaded_at":"2025-04-05T13:00:00Z"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.order)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(data))
			}
		})
	}
}

func TestOrderResponse_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected OrderResponse
	}{
		{
			name:  "with accrual",
			input: `{"number":"ORD123","status":"PROCESSING","accrual":56.78,"uploaded_at":"2025-04-05T14:00:00Z"}`,
			expected: OrderResponse{
				Number:     "ORD123",
				Status:     StatusProcessing,
				Accrual:    floatPtr(56.78),
				UploadedAt: "2025-04-05T14:00:00Z",
			},
		},
		{
			name:  "without accrual",
			input: `{"number":"ORD456","status":"NEW","uploaded_at":"2025-04-05T15:00:00Z"}`,
			expected: OrderResponse{
				Number:     "ORD456",
				Status:     StatusNew,
				Accrual:    nil,
				UploadedAt: "2025-04-05T15:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var order OrderResponse
			if err := json.Unmarshal([]byte(tt.input), &order); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if order.Number != tt.expected.Number {
				t.Errorf("Number: expected %q, got %q", tt.expected.Number, order.Number)
			}

			if order.Status != tt.expected.Status {
				t.Errorf("Status: expected %q, got %q", tt.expected.Status, order.Status)
			}

			if order.UploadedAt != tt.expected.UploadedAt {
				t.Errorf(
					"UploadedAt: expected %q, got %q",
					tt.expected.UploadedAt,
					order.UploadedAt,
				)
			}

			if (tt.expected.Accrual == nil) != (order.Accrual == nil) {
				t.Errorf(
					"Accrual nilness mismatch: expected nil=%v, got %v",
					tt.expected.Accrual == nil,
					order.Accrual == nil,
				)
			} else if tt.expected.Accrual != nil && order.Accrual != nil {
				if *tt.expected.Accrual != *order.Accrual {
					t.Errorf(
						"Accrual value mismatch: expected %v, got %v",
						*tt.expected.Accrual,
						*order.Accrual,
					)
				}
			}
		})
	}
}

func TestPendingOrderResponse_Create(t *testing.T) {
	p := PendingOrderResponse{
		ID:         1,
		Number:     "ORD123",
		Status:     StatusNew,
		UserID:     999,
		UploadedAt: time.Now(),
	}
	if p.ID != 1 || p.Number != "ORD123" || p.UserID != 999 {
		t.Errorf("PendingOrderResponse not created correctly")
	}
}

func TestOrderStatusResponse_Create(t *testing.T) {
	s := OrderStatusResponse{Status: StatusProcessing}
	if s.Status != StatusProcessing {
		t.Errorf("OrderStatusResponse not created correctly")
	}
}

func floatPtr(f float64) *float64 {
	return &f
}
