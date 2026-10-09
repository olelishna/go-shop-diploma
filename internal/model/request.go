// Package model implements models
package model

type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type AuthRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type WithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}
