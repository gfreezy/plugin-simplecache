# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Traefik middleware plugin that caches HTTP responses to disk. It's designed to work with Traefik v3.0+ as an experimental plugin.

**Module:** `github.com/gfreezy/plugin-simpleforcecache`
**Language:** Go 1.15+
**Plugin Type:** Traefik middleware

## Development Commands

### Testing
```bash
# Run all tests with coverage
go test -v -cover ./...

# Run tests with yaegi (Traefik's interpreter)
yaegi test -v .
```

### Linting
```bash
# Run golangci-lint (configuration in .golangci.toml)
make lint
# or directly:
golangci-lint run
```

### Docker Testing
```bash
# Test the plugin locally with Traefik using docker-compose
make docker
# or:
docker compose up
```

This starts Traefik with the plugin loaded and a simple hit counter service. Access at http://localhost with Traefik dashboard at http://localhost:8080.

### Vendor Management
```bash
# Create vendor directory
make vendor

# Clean vendor directory
make clean
```

## Architecture

### Core Components

1. **cache.go** - Main middleware implementation
   - `Config`: Plugin configuration struct with fields: `Path`, `MaxExpiry`, `Cleanup`, `AddStatusHeader`, `Force`, `CacheHeaders`, `CachePathPrefixes`
   - `cache`: Main handler struct that wraps the next HTTP handler
   - `ServeHTTP`: Main request handling logic - checks cache, serves cached response or passes through and caches result
   - `cacheable`: Determines if a response should be cached (only caches 200 responses and paths matching configured prefixes)
   - `matchesPathPrefix`: Helper function to check if request path matches configured prefixes (case-insensitive)
   - `cacheKey`: Generates cache key from request (Method + Host + URL.Path + configured headers with canonical names)
   - `responseWriter`: Custom response writer that captures status and body for caching

2. **file.go** - Disk-based cache storage implementation
   - `fileCache`: Manages file-based cache with vacuum goroutine for cleanup
   - `Get/Set`: Read/write cache entries with expiry timestamps (8-byte prefix)
   - `vacuum`: Background goroutine that periodically removes expired entries
   - `keyPath`: Generates hierarchical directory structure using CRC32 hash for distribution
   - `pathMutex`: Per-key locking mechanism to prevent concurrent access issues

### Cache Storage Format

- Cache files are stored in a hierarchical directory structure: `{path}/{h1}/{h2}/{h3}/{h4}/{sanitized-key}`
- Each file contains an 8-byte little-endian timestamp (expiry time) followed by JSON-encoded response data
- Response data includes: HTTP status, headers, and body

### Key Behaviors

- **Only caches 200 responses** - See `cacheable()` in cache.go:135-146
- **Path prefix filtering**: Only paths matching configured prefixes are cached (case-insensitive)
  - If `CachePathPrefixes` is empty, all paths are cached (default behavior)
  - If configured, only paths starting with one of the prefixes will be cached
  - Matching is case-insensitive: `/API/users` matches prefix `/api/`
- **Cache key**: Combination of HTTP method, host, URL path, and optionally configured request headers. Query parameters NOT included by default.
  - Base key format: `{Method}{Host}{Path}`
  - With headers: `{Method}{Host}{Path}|{Header1}:{Value1}|{Header2}:{Value2}`
  - Configure via `CacheHeaders` in config (e.g., `["Accept-Language", "X-Custom-Header"]`)
  - **Header names are case-insensitive**: Uses `http.CanonicalHeaderKey` to normalize header names
- **Expiry**: All responses cached for `maxExpiry` seconds (default 300)
- **Vacuum**: Background cleanup runs every `cleanup` seconds (default 600)
- **Concurrency**: Uses per-key RW mutexes to handle concurrent access safely
- **Cache-Status header**: Adds `hit`, `miss`, or `error` status to responses (configurable)

## Configuration

Default values (see `CreateConfig()` in cache.go):
- `maxExpiry`: 300 seconds (5 minutes)
- `cleanup`: 300 seconds (5 minutes) - Note: README says 600 but code defaults to 300
- `addStatusHeader`: true
- `force`: false (not currently implemented in caching logic)
- `cacheHeaders`: empty (no headers included in cache key by default)
- `cachePathPrefixes`: empty (all paths are cached by default)

### Cache Headers Configuration

The `cacheHeaders` field allows you to specify which HTTP request headers should be included in the cache key. This enables different cache entries for requests with different header values.

**Note**: Header names are case-insensitive. `accept-language`, `Accept-Language`, and `ACCEPT-LANGUAGE` are all treated as the same header.

Example configuration:
```yaml
cacheHeaders:
  - "Accept-Language"
  - "X-Custom-Header"
```

This will create separate cache entries for requests with different `Accept-Language` or `X-Custom-Header` values.

### Cache Path Prefixes Configuration

The `cachePathPrefixes` field allows you to specify which URL paths should be cached based on their prefix. Only requests with paths starting with one of the configured prefixes will be cached.

**Note**: Path prefix matching is case-insensitive. `/API/users` will match the prefix `/api/`.

Example configuration:
```yaml
cachePathPrefixes:
  - "/api/"
  - "/static/"
  - "/cache/"
```

With this configuration:
- `/api/users` - **cached** (matches `/api/` prefix)
- `/API/data` - **cached** (case-insensitive match with `/api/`)
- `/static/images/logo.png` - **cached** (matches `/static/` prefix)
- `/other/path` - **not cached** (doesn't match any prefix)

If `cachePathPrefixes` is empty or not specified, all paths are cached (default behavior).

## Testing Notes

- Tests use temporary directories created with `createTempDir()`
- Test coverage includes configuration validation and basic cache hit/miss scenarios
- Docker setup provides integration testing with actual Traefik instance
