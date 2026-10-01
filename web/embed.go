// Package web holds the page: its template, styles and code, embedded so
// internal/page can bundle them into each index.html.
package web

import "embed"

//go:embed index.html style.css app.js model.js layout.js render.js vendor/elk.bundled.cjs
var FS embed.FS
