// Package web exposes the embedded static assets and HTML templates that are
// compiled into the server binary at build time.
package web

import "embed"

// FS is the embedded filesystem containing the "static" and "templates"
// sub-directories. Callers can use fs.Sub to obtain a sub-tree.
//
//go:embed static templates
var FS embed.FS
