package raw

import (
	"bytes"
	"encoding/json"
	"mime"
	"strings"
)

// maxBodyLines is the number of lines of a formatted body that is shown.
const maxBodyLines = 500

// Body formats; each is also the CSS class body-<format>.
const (
	formatJSON    = "json"
	formatMetrics = "metrics"
	formatText    = "text"
)

// span is a run of body text. Class names its highlight: key, str, num, lit,
// punct, comment, name or label; empty for plain text.
type span struct {
	Class string
	Text  string
}

// line is one line of a formatted body, without its line break.
type line []span

// body is the input of the "body" view.
type body struct {
	Format string // formatJSON, formatMetrics or formatText
	Lines  []line // at most maxBodyLines
	Hidden int    // lines after Lines that are not shown
	Note   string // the body is empty or why it is shown as text; empty otherwise
}

// formatBody formats raw by the media type of contentType.
func formatBody(contentType string, raw []byte) body {
	if len(raw) == 0 {
		return body{Note: "empty body"}
	}
	switch bodyFormat(contentType) {
	case formatJSON:
		var indented bytes.Buffer
		if err := json.Indent(&indented, raw, "", "  "); err != nil {
			b := splitBody(formatText, validText(raw), plainLine)
			b.Note = "not valid JSON: " + err.Error()
			return b
		}
		return splitBody(formatJSON, validText(indented.Bytes()), jsonLine)
	case formatMetrics:
		return splitBody(formatMetrics, validText(raw), metricsLine)
	default:
		return splitBody(formatText, validText(raw), plainLine)
	}
}

// bodyFormat picks the format of a body by its Content-Type header.
func bodyFormat(contentType string) string {
	mt, params, err := mime.ParseMediaType(contentType)
	switch {
	case err != nil:
		return formatText
	case mt == "application/json", strings.HasSuffix(mt, "+json"):
		return formatJSON
	case mt == "text/plain" && params["version"] == "0.0.4", mt == "application/openmetrics-text":
		return formatMetrics
	default:
		return formatText
	}
}

func validText(b []byte) string {
	return strings.ToValidUTF8(string(b), "�")
}

// splitBody splits text into lines, drops one final line break and
// highlights the first maxBodyLines lines with highlight.
func splitBody(format, text string, highlight func(string) line) body {
	parts := strings.SplitN(strings.TrimSuffix(text, "\n"), "\n", maxBodyLines+1)
	b := body{Format: format}
	if len(parts) > maxBodyLines {
		b.Hidden = strings.Count(parts[maxBodyLines], "\n") + 1
		parts = parts[:maxBodyLines]
	}
	b.Lines = make([]line, len(parts))
	for i, p := range parts {
		b.Lines[i] = highlight(p)
	}
	return b
}

// plainLine does not highlight s.
func plainLine(s string) line {
	return line{{Text: s}}
}

// lineBuilder collects the spans of a line; adjacent plain text is merged.
type lineBuilder struct{ spans line }

func (l *lineBuilder) add(class, text string) {
	if n := len(l.spans); class == "" && n > 0 && l.spans[n-1].Class == "" {
		l.spans[n-1].Text += text
		return
	}
	l.spans = append(l.spans, span{Class: class, Text: text})
}

// quotedEnd returns the index after the closing quote of the double-quoted
// string that starts at s[start], skipping backslash escapes, or len(s) when
// the string is not closed.
func quotedEnd(s string, start int) int {
	for i := start + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return len(s)
}
