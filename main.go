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
const version = "0.1.2"

func main() {
	addr := flag.String("addr", "127.0.0.1:7860", "address to listen on (loopback only)")
	install := flag.String("install", "", "installation address (default: ANYTEXT_SELF or http://127.0.0.1:7787)")
	bucket := flag.String("bucket", "галерея", "collection with the pictures")
	flag.Parse()

	self := strings.TrimRight(strings.TrimSpace(*install), "/")
	if self == "" {
		self = strings.TrimRight(os.Getenv("ANYTEXT_SELF"), "/")
	}
	if self == "" {
		self = "http://127.0.0.1:7787"
	}

	a := &app{install: newInstallClient(self), bucket: strings.TrimSpace(*bucket)}
	log.Printf("anytext-gallery %s: %s, installation %s, collection %q", version, *addr, self, a.bucket)
	log.Fatal(http.ListenAndServe(*addr, a.routes()))
}
