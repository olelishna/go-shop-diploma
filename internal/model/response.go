package model

type ErrorResponse struct {
	Error string `json:"error"`
}

type OrderResponse struct {
	Number     string `json:"number"`
	Status     string `json:"status"`
	Accrual    *int   `json:"accrual,omitempty"`
	UploadedAt string `json:"uploaded_at"`
}

type BalanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}
