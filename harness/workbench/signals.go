package workbench

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/miroslav-matejovsky/inspector/source"
)

// pageRefreshSeconds is how often the page reloads itself.
const pageRefreshSeconds = 2

// previewBytes is the size of the body preview of each target.
const previewBytes = 512

// writeSignals writes the text view of summaries to w: a table with one row
// per target, then a preview of each non-empty latest body. It is a temporary
// view that shows the Source works, until the toolkit has a View.
func writeSignals(w io.Writer, summaries []source.TargetSummary) error {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "TARGET\tSIGNALS\tOBSERVED\tSTATUS\tDURATION\tBYTES\tERROR"); err != nil {
		return err
	}
	for _, s := range summaries {
		row := []string{s.Target.Name, strconv.FormatInt(s.Signals, 10), "-", "-", "-", "-", "-"}
		if l := s.Latest; l != nil {
			row[2] = l.ObservedAt.UTC().Format(time.RFC3339)
			if l.StatusCode != 0 {
				row[3] = strconv.Itoa(l.StatusCode)
			}
			row[4] = l.Duration.Round(time.Microsecond).String()
			row[5] = strconv.Itoa(len(l.Body))
			if l.Error != "" {
				row[6] = l.Error
			}
		}
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	for _, s := range summaries {
		if s.Latest == nil || len(s.Latest.Body) == 0 {
			continue
		}
		body := s.Latest.Body
		header := fmt.Sprintf("--- %s: %d bytes ---", s.Target.Name, len(body))
		if len(body) > previewBytes {
			header = fmt.Sprintf("--- %s: first %d of %d bytes ---", s.Target.Name, previewBytes, len(body))
			body = body[:previewBytes]
		}
		if _, err := fmt.Fprintf(w, "\n%s\n%s\n", header, strings.ToValidUTF8(string(body), "?")); err != nil {
			return err
		}
	}
	return nil
}
