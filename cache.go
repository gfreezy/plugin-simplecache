// Package plugin_simpleforcecache is a plugin to cache responses to disk.
//
//nolint:varnamelen // short variable names are acceptable
package plugin_simpleforcecache

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

// Config configures the middleware.
type Config struct {
	Path              string   `json:"path"              toml:"path"              yaml:"path"`
	MaxExpiry         int      `json:"maxExpiry"         toml:"maxExpiry"         yaml:"maxExpiry"`
	Cleanup           int      `json:"cleanup"           toml:"cleanup"           yaml:"cleanup"`
	AddStatusHeader   bool     `json:"addStatusHeader"   toml:"addStatusHeader"   yaml:"addStatusHeader"`
	Force             bool     `json:"force"             toml:"force"             yaml:"force"`
	CacheHeaders      []string `json:"cacheHeaders"      toml:"cacheHeaders"      yaml:"cacheHeaders"`
	CachePathPrefixes []string `json:"cachePathPrefixes" toml:"cachePathPrefixes" yaml:"cachePathPrefixes"`
}

// CreateConfig returns a config instance.
func CreateConfig() *Config {
	return &Config{ //nolint:exhaustruct // zero values are intentional defaults
		MaxExpiry:       int((5 * time.Minute).Seconds()),
		Cleanup:         int((5 * time.Minute).Seconds()),
		AddStatusHeader: true,
	}
}

const (
	cacheHeader      = "Cache-Status"
	cacheHitStatus   = "hit"
	cacheMissStatus  = "miss"
	cacheErrorStatus = "error"
)

type cache struct {
	name  string
	cache *fileCache
	cfg   *Config
	next  http.Handler
}

// New returns a plugin instance.
func New(_ context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	if cfg.MaxExpiry <= 1 {
		return nil, errors.New("maxExpiry must be greater or equal to 1")
	}

	if cfg.Cleanup <= 1 {
		return nil, errors.New("cleanup must be greater or equal to 1")
	}

	fc, err := newFileCache(cfg.Path, time.Duration(cfg.Cleanup)*time.Second)
	if err != nil {
		return nil, err
	}

	m := &cache{
		name:  name,
		cache: fc,
		cfg:   cfg,
		next:  next,
	}

	return m, nil
}

type cacheData struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`
}

// ServeHTTP serves an HTTP request.
//
//nolint:gocyclo,funlen // complexity and length are acceptable for main handler
func (m *cache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Skip caching if path doesn't match any configured prefix
	if !m.matchesPathPrefix(r.URL.Path) {
		m.next.ServeHTTP(w, r)
		return
	}

	cs := cacheMissStatus

	key := cacheKey(r, m.cfg.CacheHeaders)

	b, err := m.cache.Get(key)
	if err == nil {
		var data cacheData

		err := json.Unmarshal(b, &data)
		if err != nil {
			cs = cacheErrorStatus
		} else {
			for key, vals := range data.Headers {
				for _, val := range vals {
					w.Header().Add(key, val)
				}
			}

			if m.cfg.AddStatusHeader {
				w.Header().Set(cacheHeader, cacheHitStatus)
			}

			w.WriteHeader(data.Status)
			_, _ = w.Write(data.Body)

			return
		}
	}

	if m.cfg.AddStatusHeader {
		w.Header().Set(cacheHeader, cs)
	}

	rw := &responseWriter{ResponseWriter: w} //nolint:exhaustruct // zero values are intentional
	m.next.ServeHTTP(rw, r)

	expiry, ok := m.cacheable(rw.status)
	if !ok {
		return
	}

	// Filter out hop-by-hop headers that should not be cached
	headers := make(map[string][]string)

	for key, vals := range w.Header() {
		if key == "Transfer-Encoding" || key == "Connection" {
			continue
		}

		headers[key] = vals
	}

	data := cacheData{
		Status:  rw.status,
		Headers: headers,
		Body:    rw.body,
	}

	b, err = json.Marshal(data)
	if err != nil {
		log.Printf("Error serializing cache item: %v", err)
	}

	if err = m.cache.Set(key, b, expiry); err != nil { //nolint:noinlineerr // acceptable inline error
		log.Printf("Error setting cache item: %v", err)
	}
}

func (m *cache) cacheable(status int) (time.Duration, bool) {
	if status != 200 {
		return 0, false
	}

	return time.Duration(m.cfg.MaxExpiry) * time.Second, true
}

func (m *cache) matchesPathPrefix(path string) bool {
	// If no prefixes configured, cache all paths
	if len(m.cfg.CachePathPrefixes) == 0 {
		return true
	}

	lowerPath := strings.ToLower(path)
	for _, prefix := range m.cfg.CachePathPrefixes {
		if strings.HasPrefix(lowerPath, strings.ToLower(prefix)) {
			return true
		}
	}

	return false
}

func cacheKey(r *http.Request, cacheHeaders []string) string {
	var builder strings.Builder

	builder.WriteString(r.Method)
	builder.WriteString(r.Host)
	builder.WriteString(r.URL.Path)

	// Add configured headers to the cache key (case-insensitive)
	for _, headerName := range cacheHeaders {
		// Canonicalize header name to ensure case-insensitive matching
		canonicalName := http.CanonicalHeaderKey(headerName)

		headerValue := r.Header.Get(canonicalName)
		if headerValue != "" {
			builder.WriteString("|")
			builder.WriteString(canonicalName)
			builder.WriteString(":")
			builder.WriteString(headerValue)
		}
	}

	return builder.String()
}

type responseWriter struct {
	http.ResponseWriter

	status int
	body   []byte
}

func (rw *responseWriter) Write(p []byte) (int, error) {
	rw.body = append(rw.body, p...)
	return rw.ResponseWriter.Write(p)
}

func (rw *responseWriter) WriteHeader(s int) {
	rw.status = s
	rw.ResponseWriter.WriteHeader(s)
}
