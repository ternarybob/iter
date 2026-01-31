// Package web provides embedded web assets for iter-service.
package web

import "embed"

// Static contains the embedded static files (CSS, JS).
//
//go:embed static/*
var Static embed.FS

// Templates contains the embedded HTML templates.
//
//go:embed templates/*
var Templates embed.FS
