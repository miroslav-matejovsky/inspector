package explorer

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"

	"github.com/miroslav-matejovsky/inspector/model"
)

// maxRows is the number of entities shown per kind by Index and of
// backlinks shown by Entity.
const maxRows = 100

//go:embed explorer.css
var styles string

// Styles is the CSS of the explorer views. A page includes view.Styles and
// then Styles, each once.
var Styles = template.CSS(styles)

//go:embed explorer.html
var explorerHTML string

// templates holds the templates "index", "entity" and "ref".
var templates = template.Must(template.New("explorer").Parse(explorerHTML))

// ref is a related entity in a view.
type ref struct {
	Href   string // entity URL; empty when the ID is not in the model
	ID     string
	Kind   string       // empty when the ID is not in the model
	Name   string       // the ID when not in the model
	Health model.Health // model.HealthUnknown when not in the model
}

// kindSection is the table of one kind in the index.
type kindSection struct {
	Kind   string
	Count  int
	Rows   []entityRow // at most maxRows
	Hidden int         // Count - len(Rows)
}

// entityRow is one entity in the index.
type entityRow struct {
	Ref       ref
	Value     string // State.Value, "-" when empty
	Reason    string // State.Reason, "-" when empty
	Links     int
	Backlinks int
}

// relation is a link or backlink on the entity page.
type relation struct {
	Type string
	Ref  ref
}

// entityPageData is the input of the "entity" template.
type entityPageData struct {
	Entity    model.Entity
	Observed  string // Evidence.ObservedAt in RFC 3339, UTC
	Links     []relation
	Backlinks []relation // at most maxRows
	Hidden    int        // backlinks left out
}

// Index renders one section per kind, in the order in which the kinds first
// appear in m.Entities(). A section has the kind and its number of entities
// as heading and a table of its first maxRows entities: name (linked to the
// entity page), health badge, state value, reason, number of links and
// number of backlinks. The number of entities left out follows the table.
// A model without entities renders the note "no entities".
func Index(m *model.Model, entityPage string) (template.HTML, error) {
	var sections []*kindSection
	byKind := map[string]*kindSection{}
	for _, e := range m.Entities() {
		s, ok := byKind[e.Kind]
		if !ok {
			s = &kindSection{Kind: e.Kind}
			byKind[e.Kind] = s
			sections = append(sections, s)
		}
		s.Count++
		if len(s.Rows) == maxRows {
			s.Hidden++
			continue
		}
		s.Rows = append(s.Rows, entityRow{
			Ref:       newRef(m, e.ID, entityPage),
			Value:     orDash(e.State.Value),
			Reason:    orDash(e.State.Reason),
			Links:     len(e.Links),
			Backlinks: len(m.Backlinks(e.ID)),
		})
	}
	return render("index", sections)
}

// Entity renders the entity id of m: kind, name, health badge, state value
// and reason; ID and evidence (target, URL, observation time in RFC 3339);
// properties; links; and the first maxRows backlinks with the number of
// backlinks left out. Every related entity is shown with its kind, name and
// health and linked to its entity page; an ID that is not in m is shown as
// text with the note "not observed". Entity returns an error when id is not
// in m.
func Entity(m *model.Model, id, entityPage string) (template.HTML, error) {
	e, ok := m.Entity(id)
	if !ok {
		return "", fmt.Errorf("explorer: entity %q is not in the model", id)
	}
	data := entityPageData{Entity: e, Observed: e.Evidence.ObservedAt.UTC().Format(time.RFC3339)}
	for _, l := range e.Links {
		data.Links = append(data.Links, relation{Type: l.Type, Ref: newRef(m, l.To, entityPage)})
	}
	backlinks := m.Backlinks(id)
	for _, b := range backlinks[:min(len(backlinks), maxRows)] {
		data.Backlinks = append(data.Backlinks, relation{Type: b.Type, Ref: newRef(m, b.From, entityPage)})
	}
	data.Hidden = len(backlinks) - len(data.Backlinks)
	return render("entity", data)
}

// newRef returns the ref of the entity id of m. It is the only place that
// builds entity URLs.
func newRef(m *model.Model, id, entityPage string) ref {
	e, ok := m.Entity(id)
	if !ok {
		return ref{ID: id, Name: id, Health: model.HealthUnknown}
	}
	return ref{
		Href:   entityPage + "?entity=" + url.QueryEscape(id),
		ID:     id,
		Kind:   e.Kind,
		Name:   e.Name,
		Health: e.State.Health,
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// render executes the template name with data.
func render(name string, data any) (template.HTML, error) {
	var out strings.Builder
	if err := templates.ExecuteTemplate(&out, name, data); err != nil {
		return "", fmt.Errorf("explorer: render %s: %w", name, err)
	}
	// The output of html/template is escaped HTML.
	return template.HTML(out.String()), nil
}
