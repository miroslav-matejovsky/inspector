package representation_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/observation"
	"github.com/miroslav-matejovsky/inspector/representation"
)

var observedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// stubObserver returns a fixed snapshot or error and counts calls.
type stubObserver struct {
	snapshot observation.Snapshot
	err      error
	calls    int
}

func (s *stubObserver) Observe(context.Context) (observation.Snapshot, error) {
	s.calls++
	return s.snapshot, s.err
}

// fixture returns b1 and b2, books by a1, where b1 cites b2.
func fixture(t *testing.T) observation.Snapshot {
	t.Helper()
	b1 := observation.Ref{Kind: "book", ID: "b1"}
	b2 := observation.Ref{Kind: "book", ID: "b2"}
	a1 := observation.Ref{Kind: "author", ID: "a1"}
	s, err := observation.NewSnapshot(observedAt,
		[]observation.Entity{
			{Ref: b1, State: "available", Attributes: []observation.Attribute{{Name: "title", Value: "Dune"}}},
			{Ref: b2, State: "lent"},
			{Ref: a1, Attributes: []observation.Attribute{{Name: "name", Value: "Frank Herbert"}}},
		},
		[]observation.Relation{
			{From: b1, Kind: "written_by", To: a1},
			{From: b2, Kind: "written_by", To: a1},
			{From: b1, Kind: "cites", To: b2},
		})
	require.NoError(t, err)
	return s
}

func newHandler(t *testing.T, o representation.Observer) http.Handler {
	t.Helper()
	h, err := representation.NewHandler("/inspector", o)
	require.NoError(t, err)
	return h
}

func request(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func get(t *testing.T, s observation.Snapshot, target string) *httptest.ResponseRecorder {
	t.Helper()
	return request(newHandler(t, &stubObserver{snapshot: s}), http.MethodGet, target)
}

func TestNewHandlerRejectsInvalidConfig(t *testing.T) {
	for _, prefix := range []string{"", "/", "inspector", "/inspector/", "/in spector", "/in{x}"} {
		t.Run(prefix, func(t *testing.T) {
			_, err := representation.NewHandler(prefix, &stubObserver{})

			require.Error(t, err)
		})
	}
	_, err := representation.NewHandler("/inspector", nil)
	require.Error(t, err)
}

func TestOverview(t *testing.T) {
	rec := get(t, fixture(t), "/inspector/")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	require.JSONEq(t, `{
		"observed_at": "2026-01-02T03:04:05Z",
		"entities_href": "/inspector/entities",
		"kinds": [
			{"kind": "book", "count": 2, "href": "/inspector/entities?kind=book"},
			{"kind": "author", "count": 1, "href": "/inspector/entities?kind=author"}
		]
	}`, rec.Body.String())
}

func TestEntityList(t *testing.T) {
	rec := get(t, fixture(t), "/inspector/entities")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{
		"observed_at": "2026-01-02T03:04:05Z",
		"entities": [
			{"kind": "book", "id": "b1", "state": "available", "href": "/inspector/entities/book/b1"},
			{"kind": "book", "id": "b2", "state": "lent", "href": "/inspector/entities/book/b2"},
			{"kind": "author", "id": "a1", "href": "/inspector/entities/author/a1"}
		]
	}`, rec.Body.String())
}

func TestEntityListFilteredByKind(t *testing.T) {
	rec := get(t, fixture(t), "/inspector/entities?kind=book")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{
		"observed_at": "2026-01-02T03:04:05Z",
		"entities": [
			{"kind": "book", "id": "b1", "state": "available", "href": "/inspector/entities/book/b1"},
			{"kind": "book", "id": "b2", "state": "lent", "href": "/inspector/entities/book/b2"}
		]
	}`, rec.Body.String())

	rec = get(t, fixture(t), "/inspector/entities?kind=missing")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"observed_at": "2026-01-02T03:04:05Z", "entities": []}`, rec.Body.String())
}

func TestEntityDetail(t *testing.T) {
	rec := get(t, fixture(t), "/inspector/entities/book/b1")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{
		"observed_at": "2026-01-02T03:04:05Z",
		"entity": {
			"kind": "book", "id": "b1", "state": "available", "href": "/inspector/entities/book/b1",
			"attributes": [{"name": "title", "value": "Dune"}]
		},
		"related": [
			{"relation": "written_by", "direction": "outgoing",
			 "target": {"kind": "author", "id": "a1", "href": "/inspector/entities/author/a1"}},
			{"relation": "cites", "direction": "outgoing",
			 "target": {"kind": "book", "id": "b2", "href": "/inspector/entities/book/b2"}}
		]
	}`, rec.Body.String())
}

func TestEntityDetailIncomingAndEmpty(t *testing.T) {
	rec := get(t, fixture(t), "/inspector/entities/author/a1")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{
		"observed_at": "2026-01-02T03:04:05Z",
		"entity": {
			"kind": "author", "id": "a1", "href": "/inspector/entities/author/a1",
			"attributes": [{"name": "name", "value": "Frank Herbert"}]
		},
		"related": [
			{"relation": "written_by", "direction": "incoming",
			 "target": {"kind": "book", "id": "b1", "href": "/inspector/entities/book/b1"}},
			{"relation": "written_by", "direction": "incoming",
			 "target": {"kind": "book", "id": "b2", "href": "/inspector/entities/book/b2"}}
		]
	}`, rec.Body.String())

	rec = get(t, fixture(t), "/inspector/entities/book/b2")

	require.Equal(t, http.StatusOK, rec.Code)
	var detail struct {
		Entity struct{ Attributes []any }
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &detail))
	require.NotNil(t, detail.Entity.Attributes)
	require.Empty(t, detail.Entity.Attributes)
}

func TestEntityDetailEscapedID(t *testing.T) {
	s, err := observation.NewSnapshot(observedAt,
		[]observation.Entity{{Ref: observation.Ref{Kind: "book", ID: "a/b c"}}}, nil)
	require.NoError(t, err)

	rec := get(t, s, "/inspector/entities")
	require.Contains(t, rec.Body.String(), `"href":"/inspector/entities/book/a%2Fb%20c"`)

	rec = get(t, s, "/inspector/entities/book/a%2Fb%20c")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"id":"a/b c"`)
}

func TestEntityNotFound(t *testing.T) {
	rec := get(t, fixture(t), "/inspector/entities/book/zz")

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"error": {"code": "entity_not_found", "message": "entity book/zz not found"}}`,
		rec.Body.String())
}

func TestSourceUnavailable(t *testing.T) {
	h := newHandler(t, &stubObserver{err: errors.New("boom")})

	for _, target := range []string{"/inspector/", "/inspector/entities", "/inspector/entities/book/b1"} {
		t.Run(target, func(t *testing.T) {
			rec := request(h, http.MethodGet, target)

			require.Equal(t, http.StatusServiceUnavailable, rec.Code)
			var body struct {
				Error struct{ Code, Message string }
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, "source_unavailable", body.Error.Code)
			require.Contains(t, body.Error.Message, "boom")
		})
	}
}

func TestEmptySnapshot(t *testing.T) {
	s, err := observation.NewSnapshot(observedAt, nil, nil)
	require.NoError(t, err)

	rec := get(t, s, "/inspector/")
	require.JSONEq(t, `{"observed_at": "2026-01-02T03:04:05Z", "entities_href": "/inspector/entities", "kinds": []}`,
		rec.Body.String())

	rec = get(t, s, "/inspector/entities")
	require.JSONEq(t, `{"observed_at": "2026-01-02T03:04:05Z", "entities": []}`, rec.Body.String())
}

func TestUnencodableSnapshot(t *testing.T) {
	s, err := observation.NewSnapshot(time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), nil, nil)
	require.NoError(t, err)

	rec := get(t, s, "/inspector/")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), `"code":"internal_error"`)
}

func TestObservesOnEveryRequest(t *testing.T) {
	o := &stubObserver{snapshot: fixture(t)}
	h := newHandler(t, o)

	request(h, http.MethodGet, "/inspector/")
	request(h, http.MethodGet, "/inspector/")

	require.Equal(t, 2, o.calls)
}

func TestOnlyGET(t *testing.T) {
	o := &stubObserver{snapshot: fixture(t)}

	rec := request(newHandler(t, o), http.MethodPost, "/inspector/entities")

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	require.Zero(t, o.calls)
}
