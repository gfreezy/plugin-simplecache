//nolint:exhaustruct,varnamelen // test files don't need to specify all struct fields or long names
package plugin_simpleforcecache

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "should not error if path is not valid",
			cfg:     &Config{Path: fmt.Sprintf("%s/foo_%d", os.TempDir(), time.Now().Unix()), MaxExpiry: 300, Cleanup: 600},
			wantErr: false,
		},
		{
			name:    "should error if maxExpiry <= 1",
			cfg:     &Config{Path: os.TempDir(), MaxExpiry: 1, Cleanup: 600},
			wantErr: true,
		},
		{
			name:    "should error if cleanup <= 1",
			cfg:     &Config{Path: os.TempDir(), MaxExpiry: 300, Cleanup: 1},
			wantErr: true,
		},
		{
			name:    "should be valid",
			cfg:     &Config{Path: os.TempDir(), MaxExpiry: 300, Cleanup: 600},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(context.Background(), nil, test.cfg, "simplecache")

			if test.wantErr && err == nil {
				t.Fatal("expected error on bad regexp format")
			}
		})
	}
}

func TestCache_ServeHTTP(t *testing.T) {
	dir := createTempDir(t)

	next := func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Cache-Control", "max-age=20")
		rw.WriteHeader(http.StatusOK)
	}

	cfg := &Config{Path: dir, MaxExpiry: 10, Cleanup: 20, AddStatusHeader: true}

	c, err := New(context.Background(), http.HandlerFunc(next), cfg, "simplecache")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/some/path", nil)
	rw := httptest.NewRecorder()

	c.ServeHTTP(rw, req)

	if state := rw.Header().Get("Cache-Status"); state != "miss" {
		t.Errorf("unexprect cache state: want \"miss\", got: %q", state)
	}

	rw = httptest.NewRecorder()

	c.ServeHTTP(rw, req)

	if state := rw.Header().Get("Cache-Status"); state != "hit" {
		t.Errorf("unexprect cache state: want \"hit\", got: %q", state)
	}
}

func TestCache_ServeHTTP_WithHeaders(t *testing.T) {
	dir := createTempDir(t)

	callCount := 0
	next := func(rw http.ResponseWriter, _ *http.Request) {
		callCount++

		rw.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(rw, "Response %d", callCount)
	}

	cfg := &Config{
		Path:            dir,
		MaxExpiry:       10,
		Cleanup:         20,
		AddStatusHeader: true,
		CacheHeaders:    []string{"X-Custom-Header", "Accept-Language"},
	}

	c, err := New(context.Background(), http.HandlerFunc(next), cfg, "simplecache")
	if err != nil {
		t.Fatal(err)
	}

	// First request with X-Custom-Header: value1
	req1 := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
	req1.Header.Set("X-Custom-Header", "value1")
	req1.Header.Set("Accept-Language", "en-US")

	rw1 := httptest.NewRecorder()

	c.ServeHTTP(rw1, req1)

	state := rw1.Header().Get("Cache-Status")
	if state != "miss" {
		t.Errorf("unexpected cache state: want \"miss\", got: %q", state)
	}

	body := rw1.Body.String()
	if body != "Response 1" {
		t.Errorf("unexpected body: want \"Response 1\", got: %q", body)
	}

	// Second request with same headers - should be a cache hit
	req2 := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
	req2.Header.Set("X-Custom-Header", "value1")
	req2.Header.Set("Accept-Language", "en-US")

	rw2 := httptest.NewRecorder()

	c.ServeHTTP(rw2, req2)

	state = rw2.Header().Get("Cache-Status")
	if state != "hit" {
		t.Errorf("unexpected cache state: want \"hit\", got: %q", state)
	}

	body = rw2.Body.String()
	if body != "Response 1" {
		t.Errorf("unexpected body: want \"Response 1\", got: %q", body)
	}

	// Third request with different X-Custom-Header - should be a cache miss
	req3 := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
	req3.Header.Set("X-Custom-Header", "value2")
	req3.Header.Set("Accept-Language", "en-US")

	rw3 := httptest.NewRecorder()

	c.ServeHTTP(rw3, req3)

	state = rw3.Header().Get("Cache-Status")
	if state != "miss" {
		t.Errorf("unexpected cache state: want \"miss\", got: %q", state)
	}

	body = rw3.Body.String()
	if body != "Response 2" {
		t.Errorf("unexpected body: want \"Response 2\", got: %q", body)
	}

	// Fourth request with different Accept-Language - should be a cache miss
	req4 := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
	req4.Header.Set("X-Custom-Header", "value1")
	req4.Header.Set("Accept-Language", "zh-CN")

	rw4 := httptest.NewRecorder()

	c.ServeHTTP(rw4, req4)

	state = rw4.Header().Get("Cache-Status")
	if state != "miss" {
		t.Errorf("unexpected cache state: want \"miss\", got: %q", state)
	}

	body = rw4.Body.String()
	if body != "Response 3" {
		t.Errorf("unexpected body: want \"Response 3\", got: %q", body)
	}
}

func TestCache_PathPrefixes(t *testing.T) {
	dir := createTempDir(t)

	callCount := 0
	next := func(rw http.ResponseWriter, _ *http.Request) {
		callCount++

		rw.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(rw, "Response %d", callCount)
	}

	cfg := &Config{
		Path:              dir,
		MaxExpiry:         10,
		Cleanup:           20,
		AddStatusHeader:   true,
		CachePathPrefixes: []string{"/api/", "/cache/"},
	}

	c, err := New(context.Background(), http.HandlerFunc(next), cfg, "simplecache")
	if err != nil {
		t.Fatal(err)
	}

	// Test 1: Request to /api/users should be cached
	req1 := httptest.NewRequest(http.MethodGet, "http://localhost/api/users", nil)

	rw1 := httptest.NewRecorder()
	c.ServeHTTP(rw1, req1)

	state := rw1.Header().Get("Cache-Status")
	if state != "miss" {
		t.Errorf("unexpected cache state for /api/users: want \"miss\", got: %q", state)
	}

	// Test 2: Same request should be cached
	req2 := httptest.NewRequest(http.MethodGet, "http://localhost/api/users", nil)

	rw2 := httptest.NewRecorder()
	c.ServeHTTP(rw2, req2)

	state = rw2.Header().Get("Cache-Status")
	if state != "hit" {
		t.Errorf("unexpected cache state for /api/users: want \"hit\", got: %q", state)
	}

	// Test 3: Request to /cache/data should be cached (case-insensitive)
	req3 := httptest.NewRequest(http.MethodGet, "http://localhost/CACHE/data", nil)

	rw3 := httptest.NewRecorder()
	c.ServeHTTP(rw3, req3)

	state = rw3.Header().Get("Cache-Status")
	if state != "miss" {
		t.Errorf("unexpected cache state for /CACHE/data: want \"miss\", got: %q", state)
	}

	// Test 4: Request to /other/path should NOT be cached (no Cache-Status header in response)
	oldCallCount := callCount

	req4 := httptest.NewRequest(http.MethodGet, "http://localhost/other/path", nil)

	rw4 := httptest.NewRecorder()
	c.ServeHTTP(rw4, req4)

	if state := rw4.Header().Get("Cache-Status"); state != "" {
		t.Errorf("unexpected cache state for /other/path: want empty, got: %q", state)
	}

	// Make the same request again - should NOT be a cache hit
	req5 := httptest.NewRequest(http.MethodGet, "http://localhost/other/path", nil)
	rw5 := httptest.NewRecorder()
	c.ServeHTTP(rw5, req5)

	if state := rw5.Header().Get("Cache-Status"); state != "" {
		t.Errorf("unexpected cache state for /other/path second call: want empty, got: %q", state)
	}

	if callCount-oldCallCount != 2 {
		t.Errorf("expected backend to be called twice for uncached path, but was called %d times", callCount-oldCallCount)
	}
}

func TestCache_HeaderCaseInsensitive(t *testing.T) {
	dir := createTempDir(t)

	callCount := 0
	next := func(rw http.ResponseWriter, _ *http.Request) {
		callCount++

		rw.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(rw, "Response %d", callCount)
	}

	cfg := &Config{
		Path:            dir,
		MaxExpiry:       10,
		Cleanup:         20,
		AddStatusHeader: true,
		CacheHeaders:    []string{"accept-language"}, // lowercase
	}

	c, err := New(context.Background(), http.HandlerFunc(next), cfg, "simplecache")
	if err != nil {
		t.Fatal(err)
	}

	// First request with Accept-Language (different case)
	req1 := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
	req1.Header.Set("Accept-Language", "en-US")

	rw1 := httptest.NewRecorder()

	c.ServeHTTP(rw1, req1)

	state := rw1.Header().Get("Cache-Status")
	if state != "miss" {
		t.Errorf("unexpected cache state: want \"miss\", got: %q", state)
	}

	// Second request with ACCEPT-LANGUAGE (all uppercase) - should be a cache hit
	req2 := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
	req2.Header.Set("Accept-Language", "en-US")

	rw2 := httptest.NewRecorder()

	c.ServeHTTP(rw2, req2)

	state = rw2.Header().Get("Cache-Status")
	if state != "hit" {
		t.Errorf("unexpected cache state: want \"hit\", got: %q (header key should be case-insensitive)", state)
	}

	if callCount != 1 {
		t.Errorf("expected backend to be called once, but was called %d times", callCount)
	}
}

func TestCache_TransferEncodingHeader(t *testing.T) {
	dir := createTempDir(t)

	next := func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Transfer-Encoding", "chunked")
		rw.Header().Set("Content-Type", "text/plain")
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("test response"))
	}

	cfg := &Config{
		Path:            dir,
		MaxExpiry:       10,
		Cleanup:         20,
		AddStatusHeader: true,
	}

	c, err := New(context.Background(), http.HandlerFunc(next), cfg, "simplecache")
	if err != nil {
		t.Fatal(err)
	}

	// First request - cache miss
	req1 := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
	rw1 := httptest.NewRecorder()
	c.ServeHTTP(rw1, req1)

	if state := rw1.Header().Get("Cache-Status"); state != "miss" {
		t.Errorf("unexpected cache state: want \"miss\", got: %q", state)
	}

	// Second request - cache hit, Transfer-Encoding should NOT be present
	req2 := httptest.NewRequest(http.MethodGet, "http://localhost/test", nil)
	rw2 := httptest.NewRecorder()
	c.ServeHTTP(rw2, req2)

	if state := rw2.Header().Get("Cache-Status"); state != "hit" {
		t.Errorf("unexpected cache state: want \"hit\", got: %q", state)
	}

	if te := rw2.Header().Get("Transfer-Encoding"); te != "" {
		t.Errorf("Transfer-Encoding header should be filtered out from cached response, got: %q", te)
	}

	// Content-Type should still be present
	if ct := rw2.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("Content-Type header should be preserved, got: %q", ct)
	}
}

func createTempDir(tb testing.TB) string {
	tb.Helper()

	return tb.TempDir()
}
