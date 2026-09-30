package view

import (
	"fmt"
	"html/template"
	"mime"
	"strconv"
	"strings"
	"time"

	"github.com/miroslav-matejovsky/inspector/source"
)

// State classes of a target and its latest signal.
const (
	stateOK   = "ok"   // a response with a status below 400 was read
	stateBad  = "bad"  // the read failed or the status is 400 or above
	stateNone = "none" // no signal is stored
)

// rawData is the input of the "raw" template.
type rawData struct {
	Targets []targetRow
	Signals []signalView // only targets with a latest signal
}

// targetRow is one row of the "targets" view. Text fields are "-" when the
// target has no latest signal or the value does not apply.
type targetRow struct {
	Name        string
	URL         string // shown as the title of the name
	Anchor      string // id of the signal view; empty when there is none
	State       string // stateOK, stateBad or stateNone
	Signals     int64
	Observed    string
	Status      string
	Duration    string
	Size        string
	ContentType string // media type without parameters
	Error       string
}

// signalView is the input of the "signal" view.
type signalView struct {
	Name        string
	Anchor      string
	State       string
	URL         string
	Observed    string
	Status      string
	Duration    string
	Size        string
	ContentType string // full Content-Type header
	Error       string
	Body        body
}

// Raw renders the raw view of summaries as an HTML fragment: the targets
// table, then the signal view of every target with a latest signal, in the
// order of summaries. The fragment is produced by html/template, so every
// value from the inspected system is escaped. An error means a template
// failed to execute.
func Raw(summaries []source.TargetSummary) (template.HTML, error) {
	data := rawData{Targets: make([]targetRow, len(summaries))}
	for i, s := range summaries {
		data.Targets[i] = newTargetRow(s)
		if s.Latest != nil {
			data.Signals = append(data.Signals, newSignalView(s.Target.Name, *s.Latest))
		}
	}
	var out strings.Builder
	if err := templates.ExecuteTemplate(&out, "raw", data); err != nil {
		return "", fmt.Errorf("view: render raw view: %w", err)
	}
	// The output of html/template is escaped HTML.
	return template.HTML(out.String()), nil
}

func newTargetRow(s source.TargetSummary) targetRow {
	row := targetRow{
		Name: s.Target.Name, URL: s.Target.URL, State: stateNone, Signals: s.Signals,
		Observed: "-", Status: "-", Duration: "-", Size: "-", ContentType: "-", Error: "-",
	}
	if l := s.Latest; l != nil {
		v := newSignalView(s.Target.Name, *l)
		row.Anchor, row.State = v.Anchor, v.State
		row.Observed, row.Status, row.Duration, row.Size = v.Observed, v.Status, v.Duration, v.Size
		row.ContentType = mediaType(l.ContentType)
		if l.Error != "" {
			row.Error = l.Error
		}
	}
	return row
}

func newSignalView(name string, sig source.Signal) signalView {
	v := signalView{
		Name:        name,
		Anchor:      "signal-" + name,
		State:       stateOK,
		URL:         sig.URL,
		Observed:    sig.ObservedAt.UTC().Format(time.RFC3339),
		Status:      "-",
		Duration:    sig.Duration.Round(time.Microsecond).String(),
		Size:        "-",
		ContentType: sig.ContentType,
		Error:       sig.Error,
	}
	if sig.StatusCode != 0 {
		v.Status = strconv.Itoa(sig.StatusCode)
	}
	if sig.Error != "" || sig.StatusCode >= 400 {
		v.State = stateBad
	}
	if sig.Error == "" {
		v.Size = formatSize(len(sig.Body))
		v.Body = formatBody(sig.ContentType, sig.Body)
	}
	return v
}

// mediaType returns the media type of a Content-Type header without its
// parameters, the header itself when it does not parse and "-" when empty.
func mediaType(contentType string) string {
	if contentType == "" {
		return "-"
	}
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return contentType
	}
	return mt
}

// formatSize formats a byte count in B, KiB or MiB.
func formatSize(n int) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	}
}
