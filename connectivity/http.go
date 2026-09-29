package connectivity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MaxDocumentBytes bounds the body of one document.
const MaxDocumentBytes = 8 << 20

// HTTPReader reads JSON documents from one HTTP source. It sends only GET
// requests: Inspector never modifies what it inspects.
type HTTPReader struct {
	baseURL string // validated, without trailing slash
	host    string // host of baseURL
	timeout time.Duration
	client  *http.Client
}

// NewHTTPReader returns a reader for the source at baseURL, for example
// "http://127.0.0.1:8080/api". baseURL is an absolute http or https URL
// with a host, without query, fragment or trailing slash. timeout bounds
// each Get and must be positive. client must not be nil.
func NewHTTPReader(baseURL string, timeout time.Duration, client *http.Client) (*HTTPReader, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("connectivity: base URL: %w", err)
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return nil, fmt.Errorf("connectivity: base URL %q: scheme must be http or https", baseURL)
	case u.Host == "":
		return nil, fmt.Errorf("connectivity: base URL %q: host is required", baseURL)
	case u.RawQuery != "" || u.Fragment != "" || u.ForceQuery:
		return nil, fmt.Errorf("connectivity: base URL %q: query and fragment are not allowed", baseURL)
	case strings.HasSuffix(baseURL, "/"):
		return nil, fmt.Errorf("connectivity: base URL %q: trailing slash is not allowed", baseURL)
	case timeout <= 0:
		return nil, fmt.Errorf("connectivity: timeout must be positive, got %s", timeout)
	case client == nil:
		return nil, errors.New("connectivity: http client is required")
	}
	return &HTTPReader{baseURL: baseURL, host: u.Host, timeout: timeout, client: client}, nil
}

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
func (r *HTTPReader) Get(ctx context.Context, path string) (Document, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return Document{}, fmt.Errorf("connectivity: path %q must start with a single \"/\"", path)
	}
	u, err := url.Parse(r.baseURL + path)
	if err != nil {
		return Document{}, fmt.Errorf("connectivity: path %q: %w", path, err)
	}
	if u.Host != r.host {
		return Document{}, fmt.Errorf("connectivity: path %q leaves the source", path)
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Document{}, fmt.Errorf("connectivity: GET %s: %w", u, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		// *url.Error already names the method and the URL.
		return Document{}, fmt.Errorf("connectivity: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, MaxDocumentBytes+1))
	closeErr := resp.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return Document{}, fmt.Errorf("connectivity: GET %s: read body: %w", u, err)
	}
	if len(body) > MaxDocumentBytes {
		return Document{}, fmt.Errorf("connectivity: GET %s: status %d: body exceeds %d bytes",
			u, resp.StatusCode, MaxDocumentBytes)
	}
	contentType := resp.Header.Get("Content-Type")
	if mediaType, _, err := mime.ParseMediaType(contentType); err != nil || mediaType != "application/json" {
		return Document{}, fmt.Errorf("connectivity: GET %s: status %d: content type %q is not application/json",
			u, resp.StatusCode, contentType)
	}
	return Document{URL: u.String(), Status: resp.StatusCode, Body: body}, nil
}

// Decode unmarshals the body into dst. Unknown fields are ignored, so a source
// can add fields without breaking its readers.
func (d Document) Decode(dst any) error {
	if err := json.Unmarshal(d.Body, dst); err != nil {
		return fmt.Errorf("connectivity: decode %s: %w", d.URL, err)
	}
	return nil
}
