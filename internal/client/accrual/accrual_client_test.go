package accrual

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/olelishna/go-shop-diploma/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestServer(handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(handler))
}

func TestClient_GetAccrual_Success(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/orders/1234567890123456", r.URL.Path)

		resp := model.AccrualResponse{
			Order:   "1234567890123456",
			Status:  model.StatusProcessed,
			Accrual: func() *float64 { v := 15.5; return &v }(),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	resp, err := client.GetAccrual(ctx, "1234567890123456")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "1234567890123456", resp.Order)
	assert.Equal(t, model.StatusProcessed, resp.Status)
	assert.Equal(t, float64(15.5), *resp.Accrual)
}

func TestClient_GetAccrual_OrderNotFound(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	resp, err := client.GetAccrual(ctx, "999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "order not found")
	assert.Nil(t, resp)
}

func TestClient_GetAccrual_RateLimited(t *testing.T) {
	var attempt int

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}

		resp := model.AccrualResponse{
			Order:   "5556667778889990",
			Status:  model.StatusProcessed,
			Accrual: func() *float64 { v := 10.0; return &v }(),
		}

		w.Header().Set("Content-Type", "application/json")

		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	start := time.Now()
	resp, err := client.GetAccrual(ctx, "5556667778889990")
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.InDelta(t, 1*time.Second, elapsed, float64(200*time.Millisecond))
	assert.Equal(t, float64(10.0), *resp.Accrual)
}

func TestClient_GetAccrual_RetryAfter_ParseSeconds(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	_, err := client.GetAccrual(ctx, "1112223334445556")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "canceled during retry", "expected context cancellation")

	assert.Less(t, elapsed, 2*time.Second)
}

func TestClient_GetAccrual_ServerError(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetAccrual(ctx, "7778889990001112")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code 500")
}

func TestClient_GetAccrual_ContextCancelled(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Делаем медленный ответ, чтобы сработал таймаут
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client := NewClient(server.URL)

	// Очень короткий контекст
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetAccrual(ctx, "1112223334445556")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestClient_GetAccrual_OrderMismatch(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := model.AccrualResponse{
			Order:   "WRONG-ORDER",
			Status:  model.StatusProcessed,
			Accrual: func() *float64 { v := 10.0; return &v }(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetAccrual(ctx, "1112223334445556")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "received order WRONG-ORDER, expected 1112223334445556")
}

func TestClient_GetAccrual_MalformedResponse(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	_, err := client.GetAccrual(ctx, "1112223334445556")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response (status 200)")
}

func TestClient_GetAccrual_Mock(t *testing.T) {
	mockClient := NewMockAccrualClientInterface(t)

	expectedResp := &model.AccrualResponse{
		Order:   "MOCK-ORD-1",
		Status:  model.StatusProcessed,
		Accrual: func() *float64 { v := 20.0; return &v }(),
	}

	ctx := context.Background()

	mockClient.On("GetAccrual", mock.Anything, "MOCK-ORD-1").
		Return(expectedResp, nil).
		Once()

	resp, err := mockClient.GetAccrual(ctx, "MOCK-ORD-1")
	require.NoError(t, err)
	assert.Equal(t, expectedResp, resp)

	mockClient.AssertExpectations(t)
}

func TestClient_GetAccrual_Mock_Error(t *testing.T) {
	mockClient := NewMockAccrualClientInterface(t)

	ctx := context.Background()

	mockClient.On("GetAccrual", mock.Anything, "ERROR-ORD").
		Return((*model.AccrualResponse)(nil), &httpError{code: 500}).
		Once()

	resp, err := mockClient.GetAccrual(ctx, "ERROR-ORD")
	require.Error(t, err)
	assert.Nil(t, resp)

	mockClient.AssertExpectations(t)
}

type httpError struct {
	code int
}

func (e *httpError) Error() string {
	return "simulated HTTP error"
}
