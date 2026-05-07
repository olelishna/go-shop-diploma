// Package model implements requests
package model

type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type AuthRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
