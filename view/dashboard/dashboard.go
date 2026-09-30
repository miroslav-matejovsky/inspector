package dashboard

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/miroslav-matejovsky/inspector/model"
	"github.com/miroslav-matejovsky/inspector/view/chart"
)

// maxAttention is the number of entities shown under Attention.
const maxAttention = 50

//go:embed dashboard.css
var styles string

// Styles is the CSS of the dashboard. A page includes view.Styles,
// chart.Styles and then Styles, each once.
var Styles = template.CSS(styles)

//go:embed dashboard.html
var dashboardHTML string

// templates holds the template "dashboard".
var templates = template.Must(template.New("dashboard").Parse(dashboardHTML))

// dashboardData is the input of the "dashboard" template.
type dashboardData struct {
	Chart          template.HTML
	Attention      []attentionRow // at most maxAttention
	AttentionCount int            // degraded and down entities, including the hidden ones
	Hidden         int            // AttentionCount - len(Attention)
	Issues         []model.Issue
}

// attentionRow is one degraded or down entity.
type attentionRow struct {
	Href   string
	Health model.Health
	Kind   string
	Name   string
	Value  string // State.Value, "-" when empty
	Reason string // State.Reason, "-" when empty
}

// Render renders the dashboard of m: the chart.HealthByKind of m; the
// entities with health down and then degraded, each group in model order,
// at most maxAttention, with health badge, kind, name linked to its entity
// page, state value and reason; and the issues of m with their target, when
// there are any. entityPage is the path of the page that shows an entity
// for the query parameter entity.
func Render(m *model.Model, entityPage string) (template.HTML, error) {
	healthChart, err := chart.HealthByKind(m)
	if err != nil {
		return "", fmt.Errorf("dashboard: %w", err)
	}
	data := dashboardData{Chart: healthChart, Issues: m.Issues()}
	for _, h := range []model.Health{model.HealthDown, model.HealthDegraded} {
		for _, e := range m.Entities() {
			if e.State.Health != h {
				continue
			}
			data.AttentionCount++
			if len(data.Attention) == maxAttention {
				continue
			}
			data.Attention = append(data.Attention, attentionRow{
				Href:   entityPage + "?entity=" + url.QueryEscape(e.ID),
				Health: h,
				Kind:   e.Kind,
				Name:   e.Name,
				Value:  orDash(e.State.Value),
				Reason: orDash(e.State.Reason),
			})
		}
	}
	data.Hidden = data.AttentionCount - len(data.Attention)
	var out strings.Builder
	if err := templates.ExecuteTemplate(&out, "dashboard", data); err != nil {
		return "", fmt.Errorf("dashboard: render: %w", err)
	}
	// The output of html/template is escaped HTML.
	return template.HTML(out.String()), nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
