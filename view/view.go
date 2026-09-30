package view

import (
	_ "embed"
	"html/template"
)

//go:embed view.css
var styles string

// Styles is the shared CSS of every view: the palette in light and dark
// mode, the common classes and the health classes. A page that shows any
// view includes it once, before the CSS of the views.
var Styles = template.CSS(styles)
