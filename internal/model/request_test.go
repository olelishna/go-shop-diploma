package model

import (
	"encoding/json"
	"testing"
)

func TestRegisterRequest_Marshal(t *testing.T) {
	req := RegisterRequest{
		Login:    "user123",
		Password: "secret_pass",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `{"login":"user123","password":"secret_pass"}`
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestRegisterRequest_Unmarshal(t *testing.T) {
	input := `{"login":"test_user","password":"12345"}`

	var req RegisterRequest

	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if req.Login != "test_user" || req.Password != "12345" {
		t.Errorf(
			"Expected login='test_user', password='12345', got login='%s', password='%s'",
			req.Login,
			req.Password,
		)
	}
}

func TestRegisterRequest_MissingFields(t *testing.T) {
	input := `{"login":"only_login"}`

	var req RegisterRequest

	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("Unmarshal failed for partial input: %v", err)
	}

	if req.Login != "only_login" || req.Password != "" {
		t.Errorf(
			"Expected login='only_login', password='', got login='%s', password='%s'",
			req.Login,
			req.Password,
		)
	}
}

func TestAuthRequest_Marshal(t *testing.T) {
	req := AuthRequest{
		Login:    "auth_user",
		Password: "auth_pass",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `{"login":"auth_user","password":"auth_pass"}`
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestAuthRequest_Unmarshal(t *testing.T) {
	input := `{"login":"user42","password":"very_secret"}`

	var req AuthRequest

	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if req.Login != "user42" || req.Password != "very_secret" {
		t.Errorf(
			"Expected login='user42', password='very_secret', got login='%s', password='%s'",
			req.Login,
			req.Password,
		)
	}
}

func TestWithdrawRequest_Marshal(t *testing.T) {
	req := WithdrawRequest{
		Order: "ORD123",
		Sum:   99.5,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `{"order":"ORD123","sum":99.5}`
	if string(data) != expected {
		t.Errorf("Expected %s, got %s", expected, string(data))
	}
}

func TestWithdrawRequest_Unmarshal(t *testing.T) {
	input := `{"order":"ORD456","sum":250.75}`

	var req WithdrawRequest

	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if req.Order != "ORD456" || req.Sum != 250.75 {
		t.Errorf("Expected order='ORD456', sum=250.75, got order='%s', sum=%v", req.Order, req.Sum)
	}
}

func TestWithdrawRequest_Partial(t *testing.T) {
	input := `{"order":"ORD789"}`

	var req WithdrawRequest

	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("Unmarshal partial JSON failed: %v", err)
	}

	if req.Order != "ORD789" || req.Sum != 0 {
		t.Errorf("Expected order='ORD789', sum=0, got order='%s', sum=%v", req.Order, req.Sum)
	}
}
