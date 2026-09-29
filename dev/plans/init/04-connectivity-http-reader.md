---
title: "04 - Connectivity HTTP reader"
dependencies: ["01-library-boundary"]
effort: "S"
complexity: "medium"
---

# 04 - Connectivity HTTP reader

## Objective

Package `connectivity` gives Inspector read-only access to an HTTP source. `HTTPReader` sends only GET requests to paths under one base URL, bounds every request by a timeout and the body by a size limit, requires a JSON content type and returns every HTTP status to the caller as a `Document`. The package does not know the observation model or any source format.

## Target Artifacts

| File | Change |
| --- | --- |
| `connectivity/doc.go` | New: package documentation (replaces `README.md`). |
| `connectivity/README.md` | Deleted. Its purpose and core questions move to `doc.go`. |
| `connectivity/http.go` | New: `MaxDocumentBytes`, `HTTPReader`, `NewHTTPReader`, `HTTPReader.Get`, `Document`, `Document.Decode`. |
| `connectivity/http_test.go` | New tests, package `connectivity_test`. |
| `.go-arch-lint.yml` | Component `connectivity: { in: connectivity }`. No `deps` entry: stdlib only. |
| `README.md` (root) | Row `connectivity` in the `## Library` package table. |

## Implementation Tasks

1. Write `connectivity/http_test.go` with every test listed below. Confirm it fails to compile.
2. Create `connectivity/http.go`.
3. Create `connectivity/doc.go`; delete `connectivity/README.md`.
4. Add the component to `.go-arch-lint.yml`.
5. Add the table row to root `README.md`.
6. Run `go test ./connectivity/...` until it passes, then `task all`.

## Technical Details

### API (`http.go`)

```go
// MaxDocumentBytes bounds the body of one document.
const MaxDocumentBytes = 8 << 20

// HTTPReader reads JSON documents from one HTTP source. It sends only GET
// requests: Inspector never modifies what it inspects.
type HTTPReader struct {
    baseURL string        // validated, without trailing slash
    host    string        // host of baseURL
    timeout time.Duration
    client  *http.Client
}

// NewHTTPReader returns a reader for the source at baseURL, for example
// "http://127.0.0.1:8080/api". baseURL is an absolute http or https URL
// with a host, without query, fragment or trailing slash. timeout bounds
// each Get and must be positive. client must not be nil.
func NewHTTPReader(baseURL string, timeout time.Duration, client *http.Client) (*HTTPReader, error)

// Document is one JSON document read from a source.
type Document struct {
    URL    string // absolute URL that was read
    Status int    // HTTP status code
    Body   []byte // raw JSON body, at most MaxDocumentBytes
}

// Get reads the document at path, relative to the base URL. path starts with
// exactly one "/" and may contain a query. Every HTTP status is returned as a
// Document; the caller decides which statuses it accepts. Get fails when the
// path is invalid, the request fails or times out, the body exceeds
// MaxDocumentBytes, or the Content-Type is not application/json.
func (r *HTTPReader) Get(ctx context.Context, path string) (Document, error)

// Decode unmarshals the body into dst. Unknown fields are ignored, so a source
// can add fields without breaking its readers.
func (d Document) Decode(dst any) error
```

### `NewHTTPReader` validation

Each error starts with `connectivity: `.

| Input | Result |
| --- | --- |
| `url.Parse` fails | error wrapping the parse error |
| scheme is not `http` or `https` | `connectivity: base URL %q: scheme must be http or https` |
| empty host | `connectivity: base URL %q: host is required` |
| non-empty `RawQuery` or `Fragment` | `connectivity: base URL %q: query and fragment are not allowed` |
| ends with `/` | `connectivity: base URL %q: trailing slash is not allowed` |
| `timeout <= 0` | `connectivity: timeout must be positive, got %s` |
| `client == nil` | `connectivity: http client is required` |

### `Get` sequence

1. If `path` does not start with `/` or starts with `//`: `connectivity: path %q must start with a single "/"`. No request is sent.
2. `u, err := url.Parse(r.baseURL + path)`; on error return it wrapped. If `u.Host != r.host`: `connectivity: path %q leaves the source`. No request is sent.
3. `ctx, cancel := context.WithTimeout(ctx, r.timeout)`; `defer cancel()`.
4. `req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)`; `req.Header.Set("Accept", "application/json")`.
5. `resp, err := r.client.Do(req)`; on error return `fmt.Errorf("connectivity: %w", err)`. The `*url.Error` already names method and URL and unwraps to `context.DeadlineExceeded` on timeout.
6. Read the body without `defer` (golangci-lint `errcheck` rejects an unchecked deferred `Close`):

   ```go
   body, readErr := io.ReadAll(io.LimitReader(resp.Body, MaxDocumentBytes+1))
   closeErr := resp.Body.Close()
   if err := errors.Join(readErr, closeErr); err != nil {
       return Document{}, fmt.Errorf("connectivity: GET %s: read body: %w", u, err)
   }
   ```

7. If `len(body) > MaxDocumentBytes`: `connectivity: GET %s: status %d: body exceeds %d bytes`.
8. `mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))`. If `err != nil` or `mediaType != "application/json"`: `connectivity: GET %s: status %d: content type %q is not application/json`.
9. Return `Document{URL: u.String(), Status: resp.StatusCode, Body: body}`.

`Decode`: `json.Unmarshal(d.Body, dst)`; on error `connectivity: decode %s: %w` with `d.URL`.

### `doc.go`

1. First sentence: `Package connectivity is the Connectivity domain of Inspector: it gives read-only access to the sources of observations.`
2. Core questions from the deleted `README.md`: Where does the information come from? How is it accessed? How is it connected?
3. `# Read-only`: `HTTPReader` has no method that sends anything but GET. This enforces the Observe principle at the boundary.
4. `# Documents`: status handling (every status returned), JSON content type, size limit, timeout per `Get`, tolerant decoding.
5. `# Errors`: all errors are system failures of the source access; they carry the URL and the original error.

### Root `README.md` row

```markdown
| `connectivity` | Connectivity | Read-only access to sources: GET-only JSON document reader over HTTP. | stdlib |
```

### Tests (`http_test.go`, package `connectivity_test`)

Sources are `httptest.NewServer` instances with handlers written in the test, closed with `t.Cleanup`. Readers use `srv.Client()` and a 1 second timeout unless stated. A helper `jsonHandler(status int, body string)` sets `Content-Type: application/json` and writes the body.

| Test | Assertion |
| --- | --- |
| `TestNewHTTPReaderRejectsInvalidConfig` | Table, each returns an error: base `""`, `localhost:8080`, `ftp://example.test`, `http://`, `http://example.test/api/`, `http://example.test?x=1`, `http://example.test#f`; timeout `0`; client `nil`. |
| `TestGetReadsDocument` | Server handles `/base/items`, records method and query, answers 200 `{"a":1}`. Reader base `srv.URL + "/base"`; `Get(ctx, "/items?x=1")` returns `Status` 200, `Body` JSON-equal to `{"a":1}`, `URL` `srv.URL + "/base/items?x=1"`. Server saw method `GET`, query `x=1` and header `Accept: application/json`. |
| `TestGetReturnsErrorStatusAsDocument` | Server answers 503 `{"status":"down"}`. `Get` returns no error, `Status` 503, body preserved. |
| `TestGetAcceptsJSONWithCharset` | Content type `application/json; charset=utf-8` is accepted. |
| `TestGetRejectsInvalidPath` | Paths `""`, `items`, `//other.test/x`, `http://other.test/x` each return an error; a request counter on the server stays 0. |
| `TestGetRejectsNonJSON` | Server answers 404 `text/plain`. Error contains `status 404` and `text/plain`. |
| `TestGetRejectsOversizedBody` | Server writes `MaxDocumentBytes + 1` bytes with JSON content type. Error contains `exceeds`. |
| `TestGetTimesOut` | Handler blocks on `<-r.Context().Done()`. Reader timeout `50 * time.Millisecond`. `require.ErrorIs(err, context.DeadlineExceeded)`. The outcome does not depend on timing; only the test duration does. |
| `TestGetFailsWhenSourceIsDown` | Server closed before `Get`. `Get` returns an error. |
| `TestDocumentDecode` | `Document{URL: "u", Body: []byte(`{"name":"x","extra":1}`)}.Decode(&v)` with `v struct{ Name string }` gives `v.Name == "x"` and no error. Body `{` returns an error containing `decode u`. |

## Verification

```powershell
go test ./connectivity/...
go-arch-lint check
task boundary
task all
```

## Acceptance Criteria

- `go test ./connectivity/...` exits 0 and runs every test listed above.
- `connectivity/README.md` does not exist; `connectivity/doc.go` exists.
- `go list -deps ./connectivity` lists only standard library packages and `github.com/miroslav-matejovsky/inspector/connectivity`.
- `connectivity/http.go` contains no HTTP method other than `http.MethodGet` (`Select-String -Path connectivity/http.go -Pattern 'Method(Post|Put|Patch|Delete)'` finds nothing).
- `go-arch-lint check` prints `OK - No warnings found`.
- `task boundary` prints `boundary: no issues found`.
- `task all` exits 0.

## Non-Goals

- Retries, caching, authentication, TLS configuration.
- Following the source's hypermedia links. The caller chooses paths.
- Sources other than HTTP.
- Knowledge of `observation` types.
