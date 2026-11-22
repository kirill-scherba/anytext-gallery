// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// api.go — what the screen asks: which pictures are in the collection, and how the installation is.
//
// The pictures themselves are not served here: the screen takes a preview and the original from the
// installation's own endpoints (/api/thumb, /api/file), because those work at the machine and through
// the relay alike - a thumbnail is fetched with the account token the shell puts on every request
// (docs/MODULES.md, item 4).
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// app is the module with its single dependency.
type app struct {
	install *installClient
	bucket  string

	// root is the folder of the collection, read once: a gallery page must not ask the installation
	// for the collection list on every scroll step.
	root   string
	rootAt time.Time
}

// rootFolder answers the collection's folder, remembered for a minute.
func (a *app) rootFolder(ctx context.Context) string {
	if a.root != "" && time.Since(a.rootAt) < time.Minute {
		return a.root
	}
	if root, err := a.install.BucketRoot(ctx, a.bucket); err == nil {
		a.root, a.rootAt = root, time.Now()
	}
	return a.root
}

// routes is the whole surface of the module.
func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("GET /api/status", a.status)
	mux.HandleFunc("GET /api/config", a.config)
	mux.HandleFunc("GET /api/images", a.images)
	// The screen is one document: its styles and its script come inside it (ui.go explains why).
	mux.HandleFunc("GET /ui/", a.page)
	return mux
}

// config tells the screen which collection it works with.
func (a *app) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket":   a.bucket,
		"articles": "",
		"version":  version,
	})
}

// status answers whether the collection is readable at all: "no pictures" and "the installation is
// not reachable" must look different.
func (a *app) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	pics, err := a.install.Pictures(ctx, a.bucket, a.rootFolder(ctx))
	answer := map[string]any{
		"version": version,
		"install": a.install.base,
		"bucket":  a.bucket,
		"ready":   err == nil,
	}
	if err == nil {
		answer["pictures"] = len(pics)
	} else {
		answer["error"] = err.Error()
	}
	writeJSON(w, http.StatusOK, answer)
}

// images answers one page of the grid: offset, limit and an optional filter by name.
func (a *app) images(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 60
	}
	if limit > 240 {
		limit = 240
	}
	filter := strings.ToLower(strings.TrimSpace(q.Get("q")))

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	pics, err := a.install.Pictures(ctx, a.bucket, a.rootFolder(ctx))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	if filter != "" {
		kept := pics[:0]
		for _, p := range pics {
			if strings.Contains(strings.ToLower(p.Name+" "+p.Folder), filter) {
				kept = append(kept, p)
			}
		}
		pics = kept
	}

	total := len(pics)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket": a.bucket,
		"total":  total,
		"offset": offset,
		"limit":  limit,
		"items":  pics[offset:end],
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]any{"error": err.Error()})
}
