// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// media.go — the permit a browser needs to fetch a picture through the relay.
//
// A relayed picture is served only with a media permit in the query: the browser cannot put an
// Authorization header on an <img src>, so the installation asks the relay for a permit and puts it into
// the address. The window does exactly this (anytext-gui/frontend/src/api.js, signed()). The module did
// not, and in the remote client a picture that was not in the browser cache answered 401 — which looked
// like "one picture is broken, the rest work", because the rest came from the cache (23.11.2026).
//
// One permit serves every picture of the installation and lives ten minutes, so it is asked for once and
// kept until shortly before it ends.
package main

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"time"
)

// mediaCache is the permit of this installation, shared by every request the module answers.
type mediaCache struct {
	mu    sync.Mutex
	base  string
	token string
	until time.Time
}

// Media is what the screen needs to build a picture address: where the media live and the permit.
type Media struct {
	Base      string `json:"base"`
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

// media returns the permit, asking the installation for a new one when the kept one is old. The doc is
// only a door: the installation checks that the document exists before it asks the relay.
func (a *app) media(ctx context.Context, doc string, max int) (Media, bool) {
	if a.mediaCache == nil {
		a.mediaCache = &mediaCache{}
	}
	a.mediaCache.mu.Lock()
	defer a.mediaCache.mu.Unlock()
	if a.mediaCache.token != "" && time.Now().Before(a.mediaCache.until) {
		return Media{Base: a.mediaCache.base, Token: a.mediaCache.token, ExpiresIn: int(time.Until(a.mediaCache.until).Seconds())}, true
	}
	if strings.TrimSpace(doc) == "" {
		return Media{}, false
	}
	raw, err := a.install.call(ctx, "media.link", map[string]any{"bucket": a.bucket, "doc": doc, "max": max})
	if err != nil {
		return Media{}, false
	}
	var answer struct {
		URL       string `json:"url"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil || answer.URL == "" {
		return Media{}, false
	}
	parsed, err := url.Parse(answer.URL)
	if err != nil {
		return Media{}, false
	}
	token := parsed.Query().Get("mt")
	if token == "" {
		return Media{}, false
	}
	// The base is the address without the route: ".../s/<id>" for a relayed installation.
	base := answer.URL[:strings.Index(answer.URL, "/api/")]
	ttl := answer.ExpiresIn
	if ttl <= 0 {
		ttl = 600
	}
	// A permit is kept a little shorter than it lives: an address built at the last second is an
	// expired address by the time the picture is asked for.
	a.mediaCache.base, a.mediaCache.token = base, token
	a.mediaCache.until = time.Now().Add(time.Duration(ttl)*time.Second - 45*time.Second)
	return Media{Base: base, Token: token, ExpiresIn: ttl}, true
}
