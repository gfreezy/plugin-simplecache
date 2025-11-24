# Simple Cache

Simple cache plugin middleware caches responses on disk.

## Configuration

To configure this plugin you should add its configuration to the Traefik dynamic configuration as explained [here](https://docs.traefik.io/getting-started/configuration-overview/#the-dynamic-configuration).
The following snippet shows how to configure this plugin with the File provider in TOML and YAML: 

Static:

```toml
[experimental.plugins.cache]
  modulename = "github.com/gfreezy/plugin-simpleforcecache"
  version = "v0.5.0"
```

```yaml
experimental:
  plugins:
    cache:
      moduleName: "github.com/gfreezy/plugin-simpleforcecache"
      version: "v0.5.0"
```

Dynamic:

```toml
[http.middlewares]
  [http.middlewares.my-cache.plugin.cache]
    path = "/some/path/to/cache/dir"
```

```yaml
http:
  middlewares:
   my-cache:
      plugin:
        cache:
          path: /some/path/to/cache/dir
```

### Options

#### Path (`path`)

The base path that files will be created under. This must be a valid filesystem
path. If the path does not exist, it will be created.

#### Max Expiry (`maxExpiry`)

*Default: 300*

The maximum number of seconds a response can be cached for. The 
actual cache time will always be lower or equal to this.

#### Cleanup (`cleanup`)

*Default: 600*

The number of seconds to wait between cache cleanup runs.
	
#### Add Status Header (`addStatusHeader`)

*Default: true*

This determines if the cache status header `Cache-Status` will be added to the
response headers. This header can have the value `hit`, `miss` or `error`.

#### Force (`force`)

*Default: false*

This determines if responses without special cache-related headers should be
cached. If this is set to `true`, responses without special headers will be
cached for the `maxExpiry` cache time. If this is set to `false`, responses
without special cache-related headers will not be cached (as in the
[original plugin](https://github.com/traefik/plugin-simpleforcecache)). Works only
if [cachecontrol](https://github.com/pquerna/cachecontrol) unable to find any
reason not to cache the response (due to headers, method, status code, etc.).

#### Cache Headers (`cacheHeaders`)

*Default: [] (empty)*

A list of HTTP request headers to include in the cache key. This allows different cache entries for requests with different header values.

Header names are **case-insensitive**. `accept-language`, `Accept-Language`, and `ACCEPT-LANGUAGE` are all treated as the same header.

Example:
```yaml
cacheHeaders:
  - "Accept-Language"
  - "X-Custom-Header"
```

This will create separate cache entries for requests with different `Accept-Language` or `X-Custom-Header` values.

#### Cache Path Prefixes (`cachePathPrefixes`)

*Default: [] (empty, all paths are cached)*

A list of URL path prefixes to filter which paths should be cached. Only requests with paths starting with one of the configured prefixes will be cached.

Path prefix matching is **case-insensitive**. `/API/users` will match the prefix `/api/`.

Example:
```yaml
cachePathPrefixes:
  - "/api/"
  - "/static/"
  - "/cache/"
```

With this configuration:
- `/api/users` - **cached** (matches `/api/` prefix)
- `/API/data` - **cached** (case-insensitive match)
- `/static/images/logo.png` - **cached** (matches `/static/` prefix)
- `/other/path` - **not cached** (doesn't match any prefix)

If `cachePathPrefixes` is empty or not specified, all paths are cached (default behavior).
