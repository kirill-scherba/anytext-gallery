// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command anytext-gallery is the gallery module of AnyText: the pictures of one collection, a grid
// with previews, a viewer, and - later - the articles written around them.
//
// The module owns no pictures and no database. The collection belongs to the installation, the
// previews and the originals are served by the installation, and the descriptions live in its index;
// the module is a screen with a memory of its own settings (docs/MODULES.md in anytext-gui).
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
)

// version is the module's own version.
const version = "0.5.14"

func main() {
	addr := flag.String("addr", "127.0.0.1:7860", "address to listen on (loopback only)")
	install := flag.String("install", "", "installation address (default: ANYTEXT_SELF or http://127.0.0.1:7787)")
	bucket := flag.String("bucket", "галерея", "collection with the pictures")
	publicBase := flag.String("public-base", "", "address of the public copies (e.g. https://bmat.uk/i/pub)")
	articles := flag.String("articles", "мемуары", "collection with the long-form articles (empty: none)")
	flag.Parse()

	self := strings.TrimRight(strings.TrimSpace(*install), "/")
	if self == "" {
		self = strings.TrimRight(os.Getenv("ANYTEXT_SELF"), "/")
	}
	if self == "" {
		self = "http://127.0.0.1:7787"
	}

	// The words about the pictures live in memory-store, reached through the installation's gateway:
	// the module keeps no database of its own (22.11.2026).
	proxy := strings.TrimRight(strings.TrimSpace(os.Getenv("ANYTEXT_MCP_PROXY")), "/")
	if proxy == "" {
		proxy = "http://127.0.0.1:7711"
	}
	installation := newInstallClient(self)
	a := &app{
		install:    installation,
		memory:     newMemoryClient(proxy, "memory-store-mcp"),
		bucket:     strings.TrimSpace(*bucket),
		publicBase: strings.TrimSpace(*publicBase),
	}
	if strings.TrimSpace(*articles) != "" {
		a.articles = newArticlesReader(installation, strings.TrimSpace(*articles))
	}
	log.Printf("anytext-gallery %s: %s, installation %s, collection %q, articles %q",
		version, *addr, self, a.bucket, strings.TrimSpace(*articles))
	log.Fatal(http.ListenAndServe(*addr, a.routes()))
}
