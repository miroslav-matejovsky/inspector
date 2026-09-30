package chart

import (
	_ "embed"
	"fmt"
	"html/template"
	"strconv"
	"strings"

	"github.com/miroslav-matejovsky/inspector/model"
)

//go:embed chart.css
var styles string

// Styles is the CSS of the charts. A page includes view.Styles and then
// Styles, each once.
var Styles = template.CSS(styles)

//go:embed chart.html
var chartHTML string

// templates holds the template "health".
var templates = template.Must(template.New("chart").Parse(chartHTML))

// Geometry of HealthByKind, in SVG user units.
const (
	labelWidth = 160 // kind labels, left
	barWidth   = 400 // every bar, whatever the number of its entities
	countGap   = 8   // between the bar and the count
	chartWidth = labelWidth + barWidth + 80
	rowHeight  = 24
	barHeight  = 16
	baseline   = 16 // of the texts of a row, from the top of the row
)

// healthOrder is the order of the segments of a bar.
var healthOrder = []model.Health{model.HealthOK, model.HealthDegraded, model.HealthDown, model.HealthUnknown}

// healthChart is the input of the "health" template; nil Rows render the
// note "no entities".
type healthChart struct {
	Width, Height string // viewBox size
	Rows          []bar
}

// bar is the row of one kind.
type bar struct {
	Kind          string
	Total         int
	TextY, CountX string
	Segments      []segment
}

// segment is the part of a bar of one health.
type segment struct {
	Health              model.Health
	X, Y, Width, Height string
	Title               string // "<kind>: <count> <health>"
}

// HealthByKind draws one bar per kind, in the order in which the kinds first
// appear in m.Entities(). A bar is split into segments by health, in the
// order ok, degraded, down, unknown, each as wide as its share of the
// entities of the kind; empty segments are left out. The kind is the label
// on the left and the number of its entities is on the right; every
// segment has a tooltip "<kind>: <count> <health>". A legend of the four
// health values follows the chart. A model without entities renders the
// note "no entities" and no SVG.
func HealthByKind(m *model.Model) (template.HTML, error) {
	var kinds []string
	counts := map[string]map[model.Health]int{}
	for _, e := range m.Entities() {
		c, ok := counts[e.Kind]
		if !ok {
			c = map[model.Health]int{}
			counts[e.Kind] = c
			kinds = append(kinds, e.Kind)
		}
		c[e.State.Health]++
	}
	data := healthChart{Width: num(chartWidth), Height: num(float64(len(kinds) * rowHeight))}
	for i, kind := range kinds {
		data.Rows = append(data.Rows, newBar(i, kind, counts[kind]))
	}
	var out strings.Builder
	if err := templates.ExecuteTemplate(&out, "health", data); err != nil {
		return "", fmt.Errorf("chart: render health by kind: %w", err)
	}
	// The output of html/template is escaped HTML.
	return template.HTML(out.String()), nil
}

// newBar returns the bar of row i for kind with the number of entities per
// health. Segment edges come from the running count, so the widths add up
// to barWidth.
func newBar(i int, kind string, counts map[model.Health]int) bar {
	total := 0
	for _, n := range counts {
		total += n
	}
	b := bar{
		Kind:   kind,
		Total:  total,
		TextY:  num(float64(i*rowHeight + baseline)),
		CountX: num(labelWidth + barWidth + countGap),
	}
	y := num(float64(i*rowHeight + (rowHeight-barHeight)/2))
	x := float64(labelWidth)
	seen := 0
	for _, h := range healthOrder {
		n := counts[h]
		if n == 0 {
			continue
		}
		seen += n
		end := labelWidth + barWidth*float64(seen)/float64(total)
		b.Segments = append(b.Segments, segment{
			Health: h,
			X:      num(x),
			Y:      y,
			Width:  num(end - x),
			Height: num(barHeight),
			Title:  fmt.Sprintf("%s: %d %s", kind, n, h),
		})
		x = end
	}
	return b
}

// num formats a coordinate for an SVG attribute.
func num(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
