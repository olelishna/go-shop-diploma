package model

import "time"

// Order status constants.
const (
	StatusNew        = "NEW"
	StatusProcessing = "PROCESSING"
	StatusInvalid    = "INVALID"
	StatusProcessed  = "PROCESSED"
)

type OrderResponse struct {
	Number     string   `json:"number"`
	Status     string   `json:"status"`
	Accrual    *float64 `json:"accrual,omitempty"`
	UploadedAt string   `json:"uploaded_at"`
}

type PendingOrderResponse struct {
	ID         int64
	Number     string
	Status     string
	UserID     int64
	UploadedAt time.Time
}

type OrderStatusResponse struct {
	Status string
}
