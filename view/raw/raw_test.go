package raw_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/miroslav-matejovsky/inspector/source"
	"github.com/miroslav-matejovsky/inspector/view/raw"
)

var t0 = time.Date(2026, 9, 30, 1, 2, 3, 0, time.UTC)

const (
	contentJSON    = "application/json; charset=utf-8"
	contentMetrics = "text/plain; version=0.0.4; charset=utf-8; escaping=underscores"
	contentText    = "text/plain; charset=utf-8"
)

func withLatest(name string, count int64, sig source.Signal) source.TargetSummary {
	sig.Target = name
	sig.URL = "http://example.test/" + name
	return source.TargetSummary{Target: source.Target{Name: name, URL: sig.URL}, Signals: count, Latest: &sig}
}

func ok(name, contentType, body string) source.TargetSummary {
	return withLatest(name, 1, source.Signal{ObservedAt: t0, StatusCode: 200, ContentType: contentType, Body: []byte(body)})
}

func render(t *testing.T, summaries ...source.TargetSummary) string {
	t.Helper()
	out, err := raw.Render(summaries)
	require.NoError(t, err)
	return string(out)
}

// row returns the table row of the target name.
func row(t *testing.T, html, name string) string {
	t.Helper()
	start := strings.Index(html, `<tr class="target`)
	require.GreaterOrEqual(t, start, 0, "no target rows")
	for _, r := range strings.SplitAfter(html[start:], "</tr>") {
		if strings.Contains(r, ">"+name+"<") {
			return r
		}
	}
	t.Fatalf("no row for %q in %s", name, html)
	return ""
}

func TestRawListsTargets(t *testing.T) {
	out := render(t,
		withLatest("alpha", 3, source.Signal{
			ObservedAt: t0, StatusCode: 200, Duration: 1500 * time.Microsecond,
			ContentType: contentText, Body: []byte("abc"),
		}),
		source.TargetSummary{Target: source.Target{Name: "beta", URL: "http://example.test/beta"}},
	)

	alpha := row(t, out, "alpha")
	for _, cell := range []string{">3<", ">2026-09-30T01:02:03Z<", ">200<", ">1.5ms<", ">3 B<", ">text/plain<"} {
		require.Contains(t, alpha, cell)
	}
	beta := row(t, out, "beta")
	require.Contains(t, beta, `class="target none"`)
	require.Contains(t, beta, ">0<")
	require.Contains(t, out, "http://example.test/beta")
}

func TestRawMarksStatus(t *testing.T) {
	out := render(t,
		ok("up", contentText, "x"),
		withLatest("down", 1, source.Signal{ObservedAt: t0, StatusCode: 503, ContentType: contentText, Body: []byte("x")}),
		withLatest("gone", 1, source.Signal{ObservedAt: t0, Error: "connection refused"}),
	)

	require.Contains(t, row(t, out, "up"), `class="target ok"`)
	require.Contains(t, row(t, out, "down"), `class="target bad"`)
	gone := row(t, out, "gone")
	require.Contains(t, gone, `class="target bad"`)
	require.Contains(t, gone, "connection refused")
}

func TestRawFormatsSizes(t *testing.T) {
	out := render(t,
		ok("empty", contentText, ""),
		ok("kib", contentText, strings.Repeat("x", 1536)),
		ok("mib", contentText, strings.Repeat("x", 3<<20)),
	)

	require.Contains(t, row(t, out, "empty"), ">0 B<")
	require.Contains(t, row(t, out, "kib"), ">1.5 KiB<")
	require.Contains(t, row(t, out, "mib"), ">3.0 MiB<")
}

func TestRawLinksTargetsToSignals(t *testing.T) {
	out := render(t, ok("alpha", contentText, "abc"))

	require.Contains(t, out, `href="#signal-alpha"`)
	require.Contains(t, out, `id="signal-alpha"`)
}

func TestRawSkipsSignalOfTargetWithoutSignals(t *testing.T) {
	out := render(t, source.TargetSummary{Target: source.Target{Name: "beta", URL: "http://example.test/beta"}})

	require.NotContains(t, out, `signal-beta`)
}

func TestRawShowsErrorOfFailedRead(t *testing.T) {
	out := render(t, withLatest("alpha", 1, source.Signal{ObservedAt: t0, Error: "body exceeds 10 bytes"}))

	require.Contains(t, row(t, out, "alpha"), ">-<")
	require.Contains(t, out, `<p class="error">body exceeds 10 bytes</p>`)
	require.NotContains(t, out, "empty body")
}

func TestRawIndentsJSON(t *testing.T) {
	out := render(t, ok("alpha", contentJSON, `{"status":"up","n":-1.5e3,"ok":true,"x":null,"l":[1]}`+"\n"))

	require.Contains(t, out, `class="body body-json"`)
	require.Contains(t, out, `<span class="punct">{</span>`+"\n  "+`<span class="key">&#34;status&#34;</span><span class="punct">:</span> <span class="str">&#34;up&#34;</span><span class="punct">,</span>`)
	require.Contains(t, out, `<span class="num">-1.5e3</span>`)
	require.Contains(t, out, `<span class="lit">true</span>`)
	require.Contains(t, out, `<span class="lit">null</span>`)
	require.Contains(t, out, "\n    "+`<span class="num">1</span>`+"\n")
}

func TestRawKeepsEscapesInJSONStrings(t *testing.T) {
	out := render(t, ok("alpha", contentJSON, `{"q":"a\"b"}`))

	require.Contains(t, out, `<span class="str">&#34;a\&#34;b&#34;</span>`)
}

func TestRawShowsInvalidJSONAsText(t *testing.T) {
	out := render(t, ok("alpha", contentJSON, `{"broken"`))

	require.Contains(t, out, `class="body body-text"`)
	require.Contains(t, out, "not valid JSON")
	require.Contains(t, out, "{&#34;broken&#34;")
}

func TestRawHighlightsMetrics(t *testing.T) {
	body := "# HELP http_requests_total Requests.\n" +
		"# TYPE http_requests_total counter\n" +
		`http_requests_total{code="200",path="/a b"} 3` + "\n" +
		"up 1\n"

	out := render(t, ok("metrics", contentMetrics, body))

	require.Contains(t, out, `class="body body-metrics"`)
	require.Contains(t, out, `<span class="comment"># HELP http_requests_total Requests.</span>`)
	require.Contains(t, out, `<span class="name">http_requests_total</span><span class="punct">{</span>`+
		`<span class="label">code</span><span class="punct">=</span><span class="str">&#34;200&#34;</span><span class="punct">,</span>`+
		`<span class="label">path</span><span class="punct">=</span><span class="str">&#34;/a b&#34;</span><span class="punct">}</span>`+
		` <span class="num">3</span>`)
	require.Contains(t, out, `<span class="name">up</span> <span class="num">1</span>`)
}

func TestRawShowsOtherBodiesAsText(t *testing.T) {
	out := render(t, ok("alpha", "text/html", "<script>alert(1)</script>"))

	require.Contains(t, out, `class="body body-text"`)
	require.Contains(t, out, "&lt;script&gt;alert(1)&lt;/script&gt;")
	require.NotContains(t, out, "<script>")
}

func TestRawShowsEmptyBody(t *testing.T) {
	out := render(t, ok("alpha", contentJSON, ""))

	require.Contains(t, out, `<p class="note">empty body</p>`)
	require.NotContains(t, out, `class="body`)
}

func TestRawLimitsBodyLines(t *testing.T) {
	var body strings.Builder
	for i := range 600 {
		fmt.Fprintf(&body, "line %d\n", i)
	}

	out := render(t, ok("alpha", contentText, body.String()))

	require.Contains(t, out, "line 499\n")
	require.NotContains(t, out, "line 500")
	require.Contains(t, out, `<p class="note">100 more lines not shown</p>`)
}

func TestRawReplacesInvalidUTF8(t *testing.T) {
	out := render(t, ok("alpha", contentText, "a\xffb"))

	require.Contains(t, out, "a�b")
}

func TestRawPutsSignalsBelowTargets(t *testing.T) {
	out := render(t, ok("alpha", contentText, "abc"))

	table := strings.Index(out, `<table class="targets">`)
	signals := strings.Index(out, `<div class="signals">`)
	signal := strings.Index(out, `id="signal-alpha"`)
	require.GreaterOrEqual(t, table, 0)
	require.Greater(t, signals, table)
	require.Greater(t, signal, signals)
}

func TestStylesSplitSignalsInTwoColumns(t *testing.T) {
	require.Contains(t, string(raw.Styles), "grid-template-columns: repeat(2, minmax(0, 1fr))")
}

func TestRawMarksScrolledBodies(t *testing.T) {
	out := render(t, ok("alpha", contentText, "abc"))

	require.Contains(t, out, `<pre class="body body-text" data-scroll="signal-alpha-body">`)
}

func TestStylesStyleRawView(t *testing.T) {
	require.Contains(t, string(raw.Styles), ".view-raw")
}

func TestRenderMarksViewRoot(t *testing.T) {
	out := render(t, ok("alpha", contentText, "abc"))

	require.True(t, strings.HasPrefix(strings.TrimSpace(out), `<div class="view view-raw">`), out)
}

func TestStylesLeaveSharedRulesToView(t *testing.T) {
	require.NotContains(t, string(raw.Styles), "--view-ok:")
	require.NotContains(t, string(raw.Styles), ".view-raw .note")
}
