package view

import (
	_ "embed"
	"html/template"
)

//go:embed view.css
var styles string

// Styles is the CSS of the views of this package. A page that shows a view
// includes it once, in a style element.
var Styles = template.CSS(styles)

//go:embed raw.html
var rawHTML string

// templates holds the templates of the raw view and its smaller views:
// "raw", "targets", "signal" and "body".
var templates = template.Must(template.New("view").Parse(rawHTML))
