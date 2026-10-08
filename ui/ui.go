// Package ui is the View in the webapp's MVC split: the dashboard HTML template,
// embedded into the binary at build time so the server ships as a single artifact
// with no runtime file dependency. pkg/server renders these; it never writes HTML
// inline. The template carries a __CAP_TOKEN__ placeholder the server substitutes
// with the per-session operator capability token on each request.
package ui

import _ "embed"

//go:embed templates/dashboard.html
var dashboardHTML string

// Dashboard returns the dashboard page HTML, placeholder intact.
func Dashboard() string { return dashboardHTML }
