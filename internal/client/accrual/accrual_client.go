package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/olelishna/go-shop-diploma/internal/model"
	"golang.org/x/net/context/ctxhttp"
)

const (
	defaultRequestTimeout = 10 * time.Second
	maxRetries            = 3
	retryDelay            = 1 * time.Second
)

type AccrualClientInterface interface {
	GetAccrual(ctx context.Context, orderNumber string) (*model.AccrualResponse, error)
}

// Client for accrual system.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates new accrual system client.
func NewClient(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: defaultRequestTimeout},
		baseURL:    baseURL,
	}
}

// GetAccrual requests accrual points for an order.
func (c *Client) GetAccrual(
	ctx context.Context,
	orderNumber string,
) (*model.AccrualResponse, error) {
	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber)

	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {

		reqCtx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if err != nil {
			cancel()

			return nil, fmt.Errorf("failed to create request (attempt %d): %w", attempt+1, err)
		}

		resp, err := ctxhttp.Do(req.Context(), c.httpClient, req)
		if err != nil {
			cancel()

			lastErr = fmt.Errorf("network error on attempt %d: %w", attempt+1, err)

			continue
		}

		if resp.StatusCode == http.StatusNoContent {
			resp.Body.Close()
			cancel()

			return nil, fmt.Errorf("order not found")
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			waitTime := parseRetryAfter(resp)
			if waitTime <= 0 {
				_ = resp.Body.Close()

				return nil, fmt.Errorf("invalid retry-after header")
			}

			select {
			case <-ctx.Done():
				_ = resp.Body.Close()

				return nil, fmt.Errorf("canceled during retry: %w", ctx.Err())
			case <-time.After(waitTime):
			}

			continue
		}

		if resp.StatusCode >= http.StatusBadRequest {
			resp.Body.Close()
			cancel()

			return nil, fmt.Errorf(
				"unexpected status code %d for order %s",
				resp.StatusCode,
				orderNumber,
			)
		}

		var accrualResp model.AccrualResponse
		if err := json.NewDecoder(resp.Body).Decode(&accrualResp); err != nil {
			resp.Body.Close()
			cancel()

			return nil, fmt.Errorf("failed to decode response (status 200): %w", err)
		}

		resp.Body.Close()
		cancel()

		if accrualResp.Order != orderNumber {
			return nil, fmt.Errorf("received order %s, expected %s", accrualResp.Order, orderNumber)
		}

		return &accrualResp, nil
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", maxRetries, lastErr)
}

func parseRetryAfter(resp *http.Response) time.Duration {
	retryStr := resp.Header.Get("Retry-After")
	if retryStr == "" {
		return 0
	}

	if sec, err := fmt.Sscanf(retryStr, "%d", new(int)); err == nil && sec == 1 {
		var secInt int

		_, err = fmt.Sscanf(retryStr, "%d", &secInt)
		if err != nil {
			return 0
		}

		return time.Duration(secInt) * time.Second
	}

	if t, err := http.ParseTime(retryStr); err == nil {
		return time.Until(t)
	}

	return 0
}
