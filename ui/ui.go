// Package ui is the View in the webapp's MVC split: the dashboard template and
// its static assets, embedded into the binary at build time so the server ships
// as a single artifact. The page is a Go html/template (dashboard.gohtml) so the
// per-session operator capability token is injected with correct contextual
// (JS-string) escaping, not a raw string replace. CSS/JS live as static files
// under static/ and are served separately, so they are cacheable and editable on
// their own. pkg/server renders this; it never writes HTML inline.
package ui

import (
	"embed"
	"html/template"
	"io"
)

//go:embed templates/dashboard.gohtml
var dashboardSrc string

// Static holds the dashboard's CSS/JS, served by the server under /static/.
//
//go:embed static/*
var Static embed.FS

var dashboardTmpl = template.Must(template.New("dashboard").Parse(dashboardSrc))

// DashboardData is the template data for the dashboard page.
type DashboardData struct {
	CapToken string // operator capability Grant, html/template-escaped into the page
}

// RenderDashboard writes the dashboard page with the capability token escaped in.
func RenderDashboard(w io.Writer, token string) error {
	return dashboardTmpl.Execute(w, DashboardData{CapToken: token})
}
