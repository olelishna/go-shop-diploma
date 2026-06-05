package compress

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testMiddlewareGzip(
	t *testing.T,
	testCase string,
	reqFunc func() *http.Request,
	expectGzipResponse bool,
	expectGzipRequest bool,
) {
	t.Run(testCase, func(t *testing.T) {
		handler := MiddlewareGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expectGzipRequest {
				decompressed, _ := io.ReadAll(r.Body)

				if string(decompressed) != "test-body" {
					t.Errorf("expected gzipped body 'test-body', got '%s'", string(decompressed))
				}
			} else {
				body, _ := io.ReadAll(r.Body)
				r.Body.Close()

				if string(body) != "test-body" {
					t.Errorf("expected plain body, got '%s'", string(body))
				}
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`ok`))
		}))

		req := reqFunc()

		if expectGzipRequest {
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			_, _ = gz.Write([]byte("test-body"))
			_ = gz.Close()
			req.Body = io.NopCloser(&buf)
			req.Header.Set("Content-Encoding", "gzip")
		}

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		if expectGzipResponse {
			if enc := rr.Header().Get("Content-Encoding"); enc != "gzip" {
				t.Errorf("expected Content-Encoding: gzip, got '%s'", enc)
			}

			gz, err := gzip.NewReader(strings.NewReader(rr.Body.String()))
			if err != nil {
				t.Fatalf("expected gzipped response body, got plain: %v", err)
			}

			defer gz.Close()

			decompressed, _ := io.ReadAll(gz)
			if string(decompressed) != "ok" {
				t.Errorf("expected decompressed body 'ok', got '%s'", string(decompressed))
			}
		} else {
			if rr.Header().Get("Content-Encoding") != "" {
				t.Errorf(
					"expected no Content-Encoding, got '%s'",
					rr.Header().Get("Content-Encoding"),
				)
			}

			if rr.Body.String() != "ok" {
				t.Errorf("expected plain body 'ok', got '%s'", rr.Body.String())
			}
		}
	})
}

func TestMiddlewareGzip_NoAcceptEncoding(t *testing.T) {
	testMiddlewareGzip(t, "NoAcceptEncoding", func() *http.Request {
		req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader("test-body"))
		req.Header.Del("Accept-Encoding")

		return req
	}, false, false)
}

func TestMiddlewareGzip_AcceptGzip(t *testing.T) {
	testMiddlewareGzip(t, "AcceptGzip", func() *http.Request {
		req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader("test-body"))
		req.Header.Set("Accept-Encoding", "gzip")

		return req
	}, true, false)
}

func TestMiddlewareGzip_AcceptGzipAndDeflate(t *testing.T) {
	testMiddlewareGzip(t, "AcceptGzipAndDeflate", func() *http.Request {
		req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader("test-body"))
		req.Header.Set("Accept-Encoding", "gzip, deflate")

		return req
	}, true, false)
}

func TestMiddlewareGzip_AcceptDeflateOnly(t *testing.T) {
	testMiddlewareGzip(t, "AcceptDeflateOnly", func() *http.Request {
		req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader("test-body"))
		req.Header.Set("Accept-Encoding", "deflate")

		return req
	}, false, false)
}

func TestMiddlewareGzip_CompressAndDecompress(t *testing.T) {
	testMiddlewareGzip(t, "CompressAndDecompress", func() *http.Request {
		req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader("test-body"))
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Content-Encoding", "gzip")

		return req
	}, true, true)
}

func TestMiddlewareGzip_CloseResources(t *testing.T) {
	handler := MiddlewareGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("expected gzip encoding")
	}
}

func TestMiddlewareGzip_BadRequest_BadGzip(t *testing.T) {
	handler := MiddlewareGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)

		w.WriteHeader(http.StatusOK)
	}))

	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader("not gzip"))
	req.Header.Set("Content-Encoding", "gzip")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for invalid gzipped body, got %d", rr.Code)
	}
}

func TestMiddlewareGzip_WriteHeaderAndEncoding(t *testing.T) {
	handler := MiddlewareGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`created`))
	}))

	req, _ := http.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	if rr.Header().Get("X-Custom") != "value" {
		t.Errorf("expected X-Custom header preserved")
	}

	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("expected Content-Encoding: gzip")
	}
}

func TestMiddlewareGzip_Write(t *testing.T) {
	handler := MiddlewareGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	}))

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	gz, err := gzip.NewReader(strings.NewReader(rr.Body.String()))
	if err != nil {
		t.Fatalf("expected gzipped response: %v", err)
	}
	defer gz.Close()

	decompressed, _ := io.ReadAll(gz)
	if string(decompressed) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(decompressed))
	}
}

func TestMiddlewareGzip_EmptyAcceptEncoding(t *testing.T) {
	testMiddlewareGzip(t, "EmptyAcceptEncoding", func() *http.Request {
		req, _ := http.NewRequest(http.MethodGet, "/test", strings.NewReader("test-body"))
		req.Header.Set("Accept-Encoding", "")

		return req
	}, false, false)
}

func TestMiddlewareGzip_AcceptGzipCaseInsensitive(t *testing.T) {
	testMiddlewareGzip(t, "AcceptGzipCaseInsensitive", func() *http.Request {
		req, _ := http.NewRequest(http.MethodGet, "/test", strings.NewReader("test-body"))
		req.Header.Set("Accept-Encoding", "GZIP")

		return req
	}, true, false)
}

func TestMiddlewareGzip_AcceptGzipLowPriority(t *testing.T) {
	testMiddlewareGzip(t, "AcceptGzipLowPriority", func() *http.Request {
		req, _ := http.NewRequest(http.MethodGet, "/test", strings.NewReader("test-body"))
		req.Header.Set("Accept-Encoding", "deflate, gzip;q=0.8")

		return req
	}, true, false)
}
