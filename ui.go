// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ui.go — the module's screen, one self-contained document: its styles and its script travel inside
// the HTML. A module's frame is filled through the relay, and a <link> or a <script src> from that
// frame carries no account token - the relay refuses it, and the person sees bare HTML
// (docs/MODULES.md in anytext-gui, item 4).
package main

import (
	"embed"
	"html/template"
	"net/http"
	"strings"
)

//go:embed ui/index.html ui/app.js ui/style.css
var uiFS embed.FS

var pageTemplate = template.Must(template.ParseFS(uiFS, "ui/index.html"))

var (
	pageStyle  = template.CSS(mustAsset("style.css"))
	pageScript = template.JS(mustAsset("app.js"))
)

// page serves the screen with the one thing the server knows: where the module is mounted.
func (a *app) page(w http.ResponseWriter, r *http.Request) {
	prefix := r.Header.Get("X-AnyText-Prefix")
	if prefix == "" {
		prefix = "/api/module/gallery"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = pageTemplate.Execute(w, map[string]any{
		"Prefix":  strings.TrimRight(prefix, "/"),
		"Person":  r.Header.Get("X-AnyText-Person"),
		"Bucket":  a.bucket,
		"Version": version,
		"Style":   pageStyle,
		"Script":  pageScript,
	})
}

// mustAsset reads one file of the screen at start: a missing file is a broken build.
func mustAsset(name string) string {
	data, err := uiFS.ReadFile("ui/" + name)
	if err != nil {
		panic("anytext-gallery: " + err.Error())
	}
	return string(data)
}
