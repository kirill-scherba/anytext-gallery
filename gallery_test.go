// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeInstall answers the two commands the module uses.
type fakeInstall struct {
	mu     sync.Mutex
	calls  []string
	bucket any
	docs   any
	fail   bool
}

func (f *fakeInstall) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Command string `json:"command"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		f.calls = append(f.calls, req.Command)
		f.mu.Unlock()
		if f.fail {
			_, _ = w.Write([]byte(`{"error":"установка занята"}`))
			return
		}
		var result any
		switch req.Command {
		case "bucket.list":
			result = f.bucket
		case "doc.list":
			result = f.docs
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
	}
}

func newFakeClient(t *testing.T, f *fakeInstall) *installClient {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	return newInstallClient(srv.URL)
}

// collection is the answer of bucket.list: the gallery folder and a neighbour.
func collection() any {
	return []map[string]any{
		{"id": "галерея", "name": "Галерея", "folders": []string{"/srv/gallery/i"}},
		{"id": "мемуары", "name": "Мемуары", "folders": []string{"/tmp/memoirs"}},
	}
}

// documents are the answer of doc.list: pictures in the root, copies in pub/, the trash, a
// subfolder, an excluded file and two files that are not pictures at all.
func documents() any {
	return map[string]any{
		"documents": []map[string]any{
			{"key": "галерея/i/b.png", "name": "b.png", "folder": "/srv/gallery/i", "kind": "image", "has_description": true},
			{"key": "галерея/i/a.png", "name": "a.png", "folder": "/srv/gallery/i", "kind": "image"},
			{"key": "галерея/i/pub/c.png", "name": "c.png", "folder": "/srv/gallery/i/pub", "kind": "image"},
			{"key": "галерея/i/trash/d.png", "name": "d.png", "folder": "/srv/gallery/i/trash", "kind": "image"},
			{"key": "галерея/i/rinok/e.jpg", "name": "e.jpg", "folder": "/srv/gallery/i/rinok", "kind": "image"},
			{"key": "галерея/i/gone.png", "name": "gone.png", "folder": "/srv/gallery/i", "kind": "image", "state": "excluded"},
			{"key": "галерея/i/readme.md", "name": "readme.md", "folder": "/srv/gallery/i", "kind": "text"},
		},
	}
}

// TestPicturesAreTheRootImages: only pictures, only the root folder - the copies in pub/, the trash
// and a subfolder are not the gallery, and a link is not a file at all.
func TestPicturesAreTheRootImages(t *testing.T) {
	f := &fakeInstall{bucket: collection(), docs: documents()}
	c := newFakeClient(t, f)

	root, err := c.BucketRoot(context.Background(), "галерея")
	if err != nil {
		t.Fatalf("BucketRoot: %v", err)
	}
	if root != "/srv/gallery/i" {
		t.Fatalf("root = %q", root)
	}

	pics, err := c.Pictures(context.Background(), "галерея", root)
	if err != nil {
		t.Fatalf("Pictures: %v", err)
	}
	if len(pics) != 2 {
		t.Fatalf("pictures = %+v", pics)
	}
	if pics[0].Name != "a.png" || pics[1].Name != "b.png" {
		t.Errorf("order = %q, %q (want by name)", pics[0].Name, pics[1].Name)
	}
	if !pics[1].HasDesc {
		t.Errorf("the described picture lost its flag: %+v", pics[1])
	}
}

// TestUnknownCollectionIsASentence: a collection that is not there names itself.
func TestUnknownCollectionIsASentence(t *testing.T) {
	f := &fakeInstall{bucket: collection()}
	c := newFakeClient(t, f)
	_, err := c.BucketRoot(context.Background(), "нет-такой")
	if err == nil || !strings.Contains(err.Error(), "нет-такой") {
		t.Fatalf("err = %v", err)
	}
}

// TestImagesPageAndFilter: the screen asks for a page with an offset, and the filter narrows it.
func TestImagesPageAndFilter(t *testing.T) {
	f := &fakeInstall{bucket: collection(), docs: documents()}
	installation := httptest.NewServer(f.handler())
	t.Cleanup(installation.Close)
	a := &app{install: newInstallClient(installation.URL), bucket: "галерея"}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/images?offset=0&limit=1")
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Total  int       `json:"total"`
		Offset int       `json:"offset"`
		Items  []Picture `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if page.Total != 2 || len(page.Items) != 1 || page.Items[0].Name != "a.png" {
		t.Fatalf("page = %+v", page)
	}

	res, err = http.Get(srv.URL + "/api/images?q=b")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(res.Body).Decode(&page)
	res.Body.Close()
	if page.Total != 1 || page.Items[0].Name != "b.png" {
		t.Errorf("filtered = %+v", page)
	}
}

// TestStatusTellsReachability: "no pictures" and "the installation is off" must look different.
func TestStatusTellsReachability(t *testing.T) {
	f := &fakeInstall{fail: true}
	installation := httptest.NewServer(f.handler())
	t.Cleanup(installation.Close)
	a := &app{install: newInstallClient(installation.URL), bucket: "галерея"}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var st struct {
		Ready bool   `json:"ready"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.Ready || st.Error == "" {
		t.Errorf("status = %+v", st)
	}
}

// TestPageIsSelfContained: the styles and the script travel inside the page (the lesson of the
// calendar in the remote client, 21.11.2026).
func TestPageIsSelfContained(t *testing.T) {
	a := &app{install: newInstallClient("http://127.0.0.1:1"), bucket: "галерея"}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/ui/", nil)
	req.Header.Set("X-AnyText-Prefix", "/api/module/gallery")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	html := string(raw)
	for _, forbidden := range []string{`<script src=`, `<link rel="stylesheet"`} {
		if strings.Contains(html, forbidden) {
			t.Errorf("page asks the browser for a separate file (%s)", forbidden)
		}
	}
	for _, needle := range []string{`.cards`, `function renderCards`, `href="/api/module/gallery/"`} {
		if !strings.Contains(html, needle) {
			t.Errorf("page does not contain %q", needle)
		}
	}
}

// TestSortPictures: the grid shows the newest first, and "by name" is the other order a person may
// want. A picture with no time (an installation older than 0.4.18) falls back to its name.
func TestSortPictures(t *testing.T) {
	pics := []Picture{
		{Name: "b.png", ModTime: "2026-11-01T10:00:00Z"},
		{Name: "a.png", ModTime: "2026-11-03T10:00:00Z"},
		{Name: "c.png", ModTime: "2026-11-02T10:00:00Z"},
		{Name: "old.png"},
	}
	sortPictures(pics, "new")
	if pics[0].Name != "a.png" || pics[1].Name != "c.png" || pics[2].Name != "b.png" || pics[3].Name != "old.png" {
		t.Errorf("new-first order = %v", names(pics))
	}
	sortPictures(pics, "name")
	if pics[0].Name != "a.png" || pics[1].Name != "b.png" || pics[2].Name != "c.png" || pics[3].Name != "old.png" {
		t.Errorf("by-name order = %v", names(pics))
	}
}

func names(pics []Picture) []string {
	out := make([]string, 0, len(pics))
	for _, p := range pics {
		out = append(out, p.Name)
	}
	return out
}

// TestWriteRightDecidesTheButton: the installation names the rights of the person on every proxied
// request, and a viewer must not be offered an editor that would only refuse him (22.11.2026).
func TestWriteRightDecidesTheButton(t *testing.T) {
	a := &app{install: newInstallClient("http://127.0.0.1:1"), bucket: "галерея"}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)

	for _, tt := range []struct {
		rights string
		want   string
	}{
		{"read", `data-write="false"`},
		{"read,write", `data-write="true"`},
		{"read, write, admin", `data-write="true"`},
		{"", `data-write="true"`}, // no header: the person at the machine
	} {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/ui/", nil)
		if tt.rights != "" {
			req.Header.Set("X-AnyText-Rights", tt.rights)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("rights %q: %v", tt.rights, err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if !strings.Contains(string(raw), tt.want) {
			t.Errorf("rights %q: the page does not say %s", tt.rights, tt.want)
		}
	}

	// The same right decides the saving itself: the header is not an ornament.
	body := strings.NewReader(`{"name":"chapter.md","text":"# T"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/article", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AnyText-Rights", "read")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("a viewer's save answered %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

// TestSaveArticleRefusals: what must never reach the installation - a chapter cleared to nothing,
// and a save by a person whose rights do not include writing.
func TestSaveArticleRefusals(t *testing.T) {
	install := newInstallClient("http://127.0.0.1:1")
	a := &app{install: install, bucket: "галерея", articles: newArticlesReader(install, "мемуары")}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)

	for _, tt := range []struct {
		name   string
		body   string
		rights string
		want   int
	}{
		{"empty text", `{"name":"chapter.md","text":"  \n\n "}`, "write", http.StatusBadRequest},
		{"no name", `{"text":"# T"}`, "write", http.StatusBadRequest},
		{"a viewer", `{"name":"chapter.md","text":"# T"}`, "read", http.StatusForbidden},
	} {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/article", strings.NewReader(tt.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-AnyText-Rights", tt.rights)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		res.Body.Close()
		if res.StatusCode != tt.want {
			t.Errorf("%s: answered %d, want %d", tt.name, res.StatusCode, tt.want)
		}
	}
}
