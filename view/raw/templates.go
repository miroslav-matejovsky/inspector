package raw

import (
	_ "embed"
	"html/template"
)

//go:embed raw.css
var styles string

// Styles is the CSS of the raw view. A page that shows the raw view includes
// view.Styles and then Styles, each once.
var Styles = template.CSS(styles)

//go:embed raw.html
var rawHTML string

// templates holds the templates of the raw view and its smaller views:
// "raw", "targets", "signal" and "body".
var templates = template.Must(template.New("view").Parse(rawHTML))
