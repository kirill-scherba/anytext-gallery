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
	texts  map[string]string
	fail   bool
}

func (f *fakeInstall) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Command string         `json:"command"`
			Args    map[string]any `json:"args"`
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
		case "doc.text":
			// The module asks for the file itself (raw) and names the collection: a reader that puts
			// something other than a name into that argument gets nothing, and the whole articles tab
			// goes silent (26.11.2026).
			doc, _ := req.Args["doc"].(string)
			bucket, _ := req.Args["bucket"].(string)
			if bucket == "" {
				_, _ = w.Write([]byte(`{"error":"коллекция не названа"}`))
				return
			}
			result = map[string]any{"doc": doc, "bucket": bucket, "text": f.texts[doc]}
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
	a := &app{
		install: install, bucket: "галерея",
		sets:     settings{Pictures: []string{"галерея"}, Articles: "мемуары"},
		articles: newArticlesReader(install, "мемуары"),
	}
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

// TestDescribeRefusals: describing a picture is a model call, so it needs the write right and a
// document; and an installation that does not answer is named as such, not as an empty description
// (23.11.2026).
func TestDescribeRefusals(t *testing.T) {
	install := newInstallClient("http://127.0.0.1:1")
	a := &app{install: install, bucket: "галерея"}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)

	for _, tt := range []struct {
		name   string
		url    string
		rights string
		want   int
	}{
		{"a viewer", "/api/describe?doc=галерея%2Fi%2Fx.png", "read", http.StatusForbidden},
		{"no document", "/api/describe", "write", http.StatusBadRequest},
		{"installation is silent", "/api/describe?doc=галерея%2Fi%2Fx.png", "write", http.StatusBadGateway},
	} {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+tt.url, nil)
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

// TestEdgesEndpoint: the relations are asked by name, and a memory that is not configured says so
// instead of answering "no relations" — an empty list and a broken door look the same on screen
// (23.11.2026).
func TestEdgesEndpoint(t *testing.T) {
	a := &app{install: newInstallClient("http://127.0.0.1:1"), bucket: "галерея"}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/edges")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("without a name: answered %d, want %d", res.StatusCode, http.StatusBadRequest)
	}

	// A memory that is not configured is a missing list, not a broken gateway: 502 for a picture nobody
	// has linked filled the console with red (23.11.2026).
	res, err = http.Get(srv.URL + "/api/edges?name=x.png")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("without a memory: answered %d, want %d", res.StatusCode, http.StatusOK)
	}
	var answer struct {
		Edges       []Edge `json:"edges"`
		Unavailable bool   `json:"unavailable"`
	}
	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
		t.Fatalf("answer is not JSON: %v", err)
	}
	if !answer.Unavailable {
		t.Error("the answer does not say the memory was not asked")
	}
	if len(answer.Edges) != 0 {
		t.Errorf("edges = %v, want none", answer.Edges)
	}
}

// TestPictureMatches: the local search looks into the description and the tags as well as the file
// name, and every word of the query must be found (23.11.2026).
func TestPictureMatches(t *testing.T) {
	meta := Meta{Tags: "Барон, попугай", Context: "глава о модели"}
	pic := Picture{Name: "img_1.png", Description: "Верблюд на балконе с видом на город", Meta: &meta}
	for _, tt := range []struct {
		query string
		want  bool
	}{
		{"img_1", true},
		{"верблюд", true},
		{"ВЕРБЛЮД балкон", true},
		{"верблюд хинкали", false},
		{"попугай", true}, // a tag
		{"модели", true},  // the context of the record
		{"модель", false}, // no stemming: "модель" is not inside "модели", and that is honest
		{"", true},        // an empty query matches everything (the filter is not applied then)
	} {
		if got := pictureMatches(pic, strings.ToLower(tt.query)); got != tt.want {
			t.Errorf("pictureMatches(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}

// TestMatchArticleSnippet: a chapter answers by its title, its author and its text; the snippet is cut
// around the hit and carries no markdown.
func TestMatchArticleSnippet(t *testing.T) {
	article := parseArticle("chapter.md", "# Глава. Попугай\n\n**Автор:** Барон\n\nНачало текста.\n\n![Картинка](img_1.png)\n\nПотом мы говорили про граф памяти, долго и подробно.\n")

	if ok, snippet := matchArticle(article, "попугай"); !ok || snippet != "" {
		t.Errorf("title match: ok=%v snippet=%q", ok, snippet)
	}
	if ok, _ := matchArticle(article, "Барон"); !ok {
		t.Error("author match failed")
	}
	ok, snippet := matchArticle(article, "граф памяти")
	if !ok {
		t.Fatal("text match failed")
	}
	if strings.Contains(snippet, "![") || strings.Contains(snippet, "**") {
		t.Errorf("snippet carries markdown: %q", snippet)
	}
	if !strings.Contains(snippet, "граф памяти") {
		t.Errorf("snippet does not show the hit: %q", snippet)
	}
	if ok, _ := matchArticle(article, "хинкали"); ok {
		t.Error("a chapter without the word answered yes")
	}
}

// TestTrimHits: the meaning search always answers with top_k documents, so the cut is relative to the
// best hit, with a ceiling and a floor.
func TestTrimHits(t *testing.T) {
	mk := func(scores ...float64) []searchHit {
		out := make([]searchHit, len(scores))
		for i, s := range scores {
			out[i] = searchHit{Score: s, Name: "x"}
		}
		return out
	}
	if got := len(trimHits(mk(0.53, 0.52, 0.50, 0.44, 0.43, 0.35, 0.34))); got != 5 {
		t.Errorf("a clear drop keeps %d hits, want 5", got)
	}
	if got := len(trimHits(mk(0.53, 0.52, 0.52, 0.52))); got != 4 {
		t.Errorf("a flat answer keeps %d hits, want 4 (the floor)", got)
	}
	many := make([]float64, 60)
	for i := range many {
		many[i] = 0.5
	}
	if got := len(trimHits(mk(many...))); got != 24 {
		t.Errorf("an even answer keeps %d hits, want 24 (the ceiling)", got)
	}
	if got := len(trimHits(mk())); got != 0 {
		t.Errorf("an empty answer keeps %d hits", got)
	}
}

// TestSearchNeedsAQuery: an empty query is not "everything", it is a mistake.
func TestSearchNeedsAQuery(t *testing.T) {
	a := &app{install: newInstallClient("http://127.0.0.1:1"), bucket: "галерея"}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/search?in=pictures")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("answered %d, want %d", res.StatusCode, http.StatusBadRequest)
	}
}

// TestSettingsFileOverridesFlags: the flags are the defaults, the file the window writes is the
// choice. A module that ignored the file would lose the collections on every restart (26.11.2026).
func TestSettingsFileOverridesFlags(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.json"
	want := settings{Pictures: []string{"галерея", " images ", "галерея"}, Articles: "мемуары", PublicBase: "https://bmat.uk/i/pub/"}
	if err := saveSettings(path, want); err != nil {
		t.Fatalf("saveSettings: %v", err)
	}
	got, ok := loadSettings(path)
	if !ok {
		t.Fatal("the file written a moment ago was not read")
	}
	if strings.Join(got.Pictures, ",") != "галерея,images" {
		t.Errorf("pictures = %v, want the duplicates and spaces gone", got.Pictures)
	}
	if got.Articles != "мемуары" || got.PublicBase != "https://bmat.uk/i/pub" {
		t.Errorf("settings = %+v", got)
	}
	if _, ok := loadSettings(dir + "/missing.json"); ok {
		t.Error("a missing file was read as a choice")
	}
}

// TestSettingsValidation: a gallery with no collection has nothing to show, and saying so before
// saving is cheaper than an empty screen after a restart.
func TestSettingsValidation(t *testing.T) {
	if err := (settings{Pictures: []string{"  "}}).clean().validate(); err == nil {
		t.Error("an empty choice was accepted")
	}
	if err := (settings{Pictures: []string{"галерея"}}).clean().validate(); err != nil {
		t.Errorf("a valid choice was refused: %v", err)
	}
}

// TestSettingsNeedsTheWriteRight: settings are the owner's, and a viewer must not change what
// everybody sees.
func TestSettingsNeedsTheWriteRight(t *testing.T) {
	a := &app{install: newInstallClient("http://127.0.0.1:1"), bucket: "галерея"}
	srv := httptest.NewServer(a.routes())
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/settings", strings.NewReader(`{"pictures":["галерея"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AnyText-Rights", "read")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("a viewer's save answered %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

// TestBucketOfDoc: a picture is described, published and thrown away in the collection its key
// starts with, not in the first one of the settings.
func TestBucketOfDoc(t *testing.T) {
	a := &app{bucket: "галерея", sets: settings{Pictures: []string{"галерея", "images"}}}
	for _, tt := range []struct{ doc, want string }{
		{"images/i/x.png", "images"},
		{"галерея/i/x.png", "галерея"},
		{"plain.png", "галерея"}, // no collection in the key: the first one
	} {
		if got := a.bucketOf(tt.doc); got != tt.want {
			t.Errorf("bucketOf(%q) = %q, want %q", tt.doc, got, tt.want)
		}
	}
}

// TestArticlesReadTheirCollection: the list asks for the collection by name and reads every chapter.
// A reader that passes something else as the collection answers "no articles" with no error at all,
// which was the whole of 0.6.1 (26.11.2026).
func TestArticlesReadTheirCollection(t *testing.T) {
	chapter := "# Глава. Проверка\n\n**Автор:** Барон\n**Дата:** 2026-11-26\n\nТекст главы.\n"
	f := &fakeInstall{
		bucket: collection(),
		docs: map[string]any{"documents": []map[string]any{
			{"key": "мемуары/memoirs/chapter.md", "name": "chapter.md", "kind": "text"},
			{"key": "мемуары/memoirs/readme.md", "name": "readme.md", "kind": "text"},
		}},
		texts: map[string]string{"мемуары/memoirs/chapter.md": chapter},
	}
	install := newFakeClient(t, f)
	reader := newArticlesReaderDynamic(install, func() string { return "мемуары" })

	list, err := reader.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("articles = %d, want 1 (a readme is not a chapter)", len(list))
	}
	if list[0].Title != "Глава. Проверка" || list[0].Author != "Барон" {
		t.Errorf("article = %+v", list[0])
	}
}

// TestArticlesFollowTheCollection: choosing another collection takes effect without a restart, and
// the cached list of the old one is dropped.
func TestArticlesFollowTheCollection(t *testing.T) {
	bucket := "мемуары"
	f := &fakeInstall{
		bucket: collection(),
		docs:   map[string]any{"documents": []map[string]any{}},
		texts:  map[string]string{},
	}
	install := newFakeClient(t, f)
	reader := newArticlesReaderDynamic(install, func() string { return bucket })

	if list, err := reader.List(context.Background()); err != nil || len(list) != 0 {
		t.Fatalf("first read: %v / %d", err, len(list))
	}
	bucket = ""
	list, err := reader.List(context.Background())
	if err != nil {
		t.Fatalf("empty collection: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("articles = %d, want none", len(list))
	}
}
