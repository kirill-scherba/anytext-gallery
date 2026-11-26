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
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// app is the module with its two dependencies: the installation (files and pictures) and memory-store
// (the words about them).
type app struct {
	install *installClient
	memory  *memoryClient
	// bucket is the first collection of pictures: the one a request that names no document belongs
	// to. Pictures themselves always carry their own collection (26.11.2026).
	bucket string
	// publicBase is the address the public copies are served at (the old gallery's /i/pub/).
	publicBase string
	// articles is the reader of the long-form collection (the memoirs), when one is configured.
	articles *articlesReader

	// roots are the folders of the collections, read once each: a gallery page must not ask the
	// installation for the collection list on every scroll step.
	roots map[string]rootEntry

	// edges is what the memory graph said about the pictures seen recently.
	edges *edgesCache
	// mediaCache is the relay's permit for picture addresses, kept until it nearly ends.
	mediaCache *mediaCache

	// mu guards the settings that a person may change while the module runs.
	mu sync.RWMutex
	// sets is what the owner chose: the pictures collections, the articles collection, the public
	// address. The flags are only the defaults until the window says otherwise.
	sets     settings
	setsPath string
}

// rootFolder answers the collection's folder, remembered for a minute. Every collection has its own:
// the grid may show several at once, and the folder of one is not the folder of another.
func (a *app) rootFolder(ctx context.Context, bucket string) string {
	if bucket == "" {
		bucket = a.mainBucket()
	}
	a.mu.RLock()
	if a.roots == nil {
		a.mu.RUnlock()
		a.mu.Lock()
		if a.roots == nil {
			a.roots = map[string]rootEntry{}
		}
		a.mu.Unlock()
		a.mu.RLock()
	}
	if e, ok := a.roots[bucket]; ok && time.Since(e.at) < time.Minute {
		a.mu.RUnlock()
		return e.root
	}
	a.mu.RUnlock()
	root, err := a.install.BucketRoot(ctx, bucket)
	if err != nil {
		return ""
	}
	a.mu.Lock()
	a.roots[bucket] = rootEntry{root: root, at: time.Now()}
	a.mu.Unlock()
	return root
}

// rootEntry is one collection's folder and when it was read.
type rootEntry struct {
	root string
	at   time.Time
}

// mainBucket is the first collection of pictures: the default for requests that name no document.
func (a *app) mainBucket() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.sets.Pictures) > 0 {
		return a.sets.Pictures[0]
	}
	return a.bucket
}

// pictureBuckets answers every collection the grid shows.
func (a *app) pictureBuckets() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.sets.Pictures) > 0 {
		return append([]string{}, a.sets.Pictures...)
	}
	if a.bucket != "" {
		return []string{a.bucket}
	}
	return nil
}

// articlesBucket answers the collection of chapters, if one is configured.
func (a *app) articlesBucket() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sets.Articles
}

// publicBaseNow is the address public copies are served at.
func (a *app) publicBaseNow() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.sets.PublicBase != "" {
		return a.sets.PublicBase
	}
	return a.publicBase
}

// bucketOf reads the collection out of a document key: a key starts with its collection, and a picture
// of the second collection must be described, published and thrown away in the second one.
func (a *app) bucketOf(doc string) string {
	if i := strings.Index(doc, "/"); i > 0 {
		return doc[:i]
	}
	return a.mainBucket()
}

// routes is the whole surface of the module.
func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("GET /api/status", a.status)
	mux.HandleFunc("GET /api/config", a.config)
	mux.HandleFunc("GET /api/settings", a.settingsGet)
	mux.HandleFunc("POST /api/settings", a.settingsPost)
	mux.HandleFunc("GET /api/images", a.images)
	mux.HandleFunc("GET /api/meta", a.meta)
	mux.HandleFunc("POST /api/describe", a.describePicture)
	mux.HandleFunc("GET /api/edges", a.pictureEdges)
	mux.HandleFunc("GET /api/signed", a.signedMedia)
	mux.HandleFunc("POST /api/meta", a.saveMeta)
	mux.HandleFunc("DELETE /api/meta", a.deleteMeta)
	mux.HandleFunc("POST /api/migrate", a.migrate)
	mux.HandleFunc("GET /api/public", a.publicState)
	mux.HandleFunc("POST /api/public", a.makePublic)
	mux.HandleFunc("DELETE /api/public", a.removePublic)
	mux.HandleFunc("POST /api/trash", a.trash)
	mux.HandleFunc("GET /api/articles", a.articlesList)
	mux.HandleFunc("GET /api/search", a.semanticSearch)
	mux.HandleFunc("GET /api/article", a.article)
	mux.HandleFunc("POST /api/article", a.saveArticle)
	// Publication of an article: the buttons live here, in the article, because that is where a
	// person reads and decides. The state itself belongs to the document in the installation.
	mux.HandleFunc("GET /api/publish", a.publishArticle)
	mux.HandleFunc("POST /api/publish", a.publishArticle)
	mux.HandleFunc("DELETE /api/publish", a.publishArticle)
	// The screen is one document: its styles and its script come inside it (ui.go explains why).
	mux.HandleFunc("GET /ui/", a.page)
	return mux
}

// errNoDoc is the one missing argument of the meta endpoints.
var errNoDoc = errors.New("нужен документ (doc)")

// config tells the screen which collection it works with.
func (a *app) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket":   a.mainBucket(),
		"buckets":  a.pictureBuckets(),
		"articles": a.articlesBucket(),
		"version":  version,
	})
}

// status answers whether the collection is readable at all: "no pictures" and "the installation is
// not reachable" must look different.
func (a *app) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	pics, err := a.pictures(ctx)
	answer := map[string]any{
		"version":  version,
		"install":  a.install.base,
		"bucket":   a.mainBucket(),
		"buckets":  a.pictureBuckets(),
		"articles": a.articlesBucket(),
		"ready":    err == nil,
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
	mode := q.Get("sort")
	if mode != "name" {
		mode = "new"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	pics := []Picture{}
	problems := []string{}
	for _, bucket := range a.pictureBuckets() {
		found, err := a.install.Pictures(ctx, bucket, a.rootFolder(ctx, bucket))
		if err != nil {
			// One unreadable collection must not hide the others: the grid says which one failed and
			// keeps showing what it has (26.11.2026).
			problems = append(problems, bucket+": "+err.Error())
			continue
		}
		for i := range found {
			found[i].Bucket = bucket
		}
		pics = append(pics, found...)
	}
	if len(problems) > 0 && len(pics) == 0 {
		writeError(w, http.StatusBadGateway, errors.New(strings.Join(problems, "; ")))
		return
	}
	// The words about the pictures are loaded before the filter: a search looks into the description
	// and the tags as well as into the file name, and without them a described picture is found only
	// by the name a person cannot remember (23.11.2026).
	a.decorate(ctx, pics)
	if filter != "" {
		kept := pics[:0]
		for _, p := range pics {
			if pictureMatches(p, filter) {
				kept = append(kept, p)
			}
		}
		pics = kept
	}
	sortPictures(pics, mode)

	total := len(pics)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := pics[offset:end]
	answer := map[string]any{
		"bucket":  a.mainBucket(),
		"buckets": a.pictureBuckets(),
		"total":   total,
		"offset":  offset,
		"limit":   limit,
		"items":   page,
	}
	// A relayed installation serves pictures only with a media permit, and the browser cannot put a
	// header on an <img src>: the permit travels with the page (23.11.2026).
	// The permit is for a browser that reaches the installation through the relay. On the machine that
	// holds the installation the pictures are local, and sending them through the relay instead would be
	// a detour for nothing (23.11.2026).
	if len(page) > 0 && wantsMedia(r) {
		if media, ok := a.media(ctx, page[0].Doc, 400); ok {
			answer["media"] = media
		}
	}
	writeJSON(w, http.StatusOK, answer)
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

// meta answers what a person wrote about one picture. The installation's own description (the index)
// says whether the copy there exists.
func (a *app) meta(w http.ResponseWriter, r *http.Request) {
	doc := r.URL.Query().Get("doc")
	if doc == "" {
		writeError(w, http.StatusBadRequest, errNoDoc)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	meta := Meta{}
	if a.memory != nil {
		if got, err := a.memory.Get(ctx, nameOf(doc)); err == nil {
			meta = got
		} else {
			writeError(w, http.StatusBadGateway, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc, "meta": meta})
}

// saveMeta writes the words into memory-store and copies the description into the installation's
// standard place: the old gallery keeps working, and the search starts finding pictures by their
// descriptions (22.11.2026 — «не переносить, а копировать»).
func (a *app) saveMeta(w http.ResponseWriter, r *http.Request) {
	doc := r.URL.Query().Get("doc")
	if doc == "" {
		writeError(w, http.StatusBadRequest, errNoDoc)
		return
	}
	var meta Meta
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&meta); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("тело запроса не разобрано: %w", err))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	answer := map[string]any{"doc": doc, "meta": meta}
	if a.memory != nil {
		if err := a.memory.Save(ctx, nameOf(doc), meta); err != nil {
			writeError(w, http.StatusBadGateway, fmt.Errorf("память: %w", err))
			return
		}
		answer["memory"] = "ok"
	}

	// The installation is where a description lives: "Describe with a model" writes there, the window
	// edits it there, and the gallery shows that text. An emptied description is dropped, not saved as
	// an empty one.
	if strings.TrimSpace(meta.Description) != "" {
		_, err := a.install.call(ctx, "doc.save_description", map[string]any{
			"bucket": a.bucketOf(doc), "doc": doc, "text": meta.Description, "model": "gallery",
		})
		if err != nil {
			answer["index"] = err.Error()
		} else {
			answer["index"] = "ok"
		}
	} else {
		if _, err := a.install.call(ctx, "doc.drop_description", map[string]any{"bucket": a.bucketOf(doc), "doc": doc}); err != nil {
			answer["index"] = err.Error()
		} else {
			answer["index"] = "удалено"
		}
	}
	writeJSON(w, http.StatusOK, answer)
}

// deleteMeta removes the words about a picture from memory-store. The copy in the installation stays:
// a description there is search, and the gallery does not own the search.
func (a *app) deleteMeta(w http.ResponseWriter, r *http.Request) {
	doc := r.URL.Query().Get("doc")
	if doc == "" {
		writeError(w, http.StatusBadRequest, errNoDoc)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if a.memory != nil {
		if err := a.memory.Delete(ctx, nameOf(doc)); err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc, "deleted": true})
}

// nameOf is the file name of a document key: the meta keys are built from names, as before.
func nameOf(doc string) string {
	if i := strings.LastIndex(doc, "/"); i >= 0 {
		return doc[i+1:]
	}
	return doc
}

// migrate copies the gallery's descriptions into the installation's standard place: the old gallery
// keeps reading memory-store, and the search starts finding pictures by what is written about them.
// It is idempotent — a picture whose description is already there is written again, which costs one
// ingest and changes nothing.
func (a *app) migrate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()
	names, err := a.memory.Names(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	docs, err := a.pictures(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	byName := make(map[string]string, len(docs))
	for _, d := range docs {
		byName[d.Name] = d.Doc
	}

	copied, skipped, failed := 0, 0, 0
	problems := []string{}
	for _, name := range names {
		doc, ok := byName[name]
		if !ok {
			skipped++ // a picture that is no longer in the collection
			continue
		}
		meta, err := a.memory.Get(ctx, name)
		if err != nil || strings.TrimSpace(meta.Description) == "" {
			skipped++
			continue
		}
		if _, err := a.install.call(ctx, "doc.save_description", map[string]any{
			"bucket": a.bucketOf(doc), "doc": doc, "text": meta.Description, "model": "gallery",
		}); err != nil {
			failed++
			if len(problems) < 10 {
				problems = append(problems, name+": "+err.Error())
			}
			continue
		}
		copied++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"records": len(names), "copied": copied, "skipped": skipped, "failed": failed, "problems": problems,
	})
}

// publicState answers whether the public copy of a picture exists.
func (a *app) publicState(w http.ResponseWriter, r *http.Request) {
	pic, err := a.picture(r.URL.Query().Get("doc"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": pic.Doc, "public": IsPublic(pic), "url": a.publicURL(pic)})
}

// makePublic copies the picture into the pub folder beside it: the old gallery's "Make public".
func (a *app) makePublic(w http.ResponseWriter, r *http.Request) {
	pic, err := a.picture(r.URL.Query().Get("doc"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if _, err := MakePublic(pic); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": pic.Doc, "public": true, "url": a.publicURL(pic)})
}

// removePublic deletes the public copy; the picture itself stays.
func (a *app) removePublic(w http.ResponseWriter, r *http.Request) {
	pic, err := a.picture(r.URL.Query().Get("doc"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err := RemovePublic(pic); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": pic.Doc, "public": false})
}

// trash takes the picture out of the gallery: the installation excludes the file from its index and
// keeps it on disk, so "delete" is reversible (the old gallery moved the file to trash/ instead).
func (a *app) trash(w http.ResponseWriter, r *http.Request) {
	doc := r.URL.Query().Get("doc")
	if doc == "" {
		writeError(w, http.StatusBadRequest, errNoDoc)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if _, err := a.install.call(ctx, "doc.delete", map[string]any{
		"bucket": a.bucketOf(doc), "doc": doc, "delete_file": false,
	}); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc, "trashed": true})
}

// picture finds one picture of the collection by its document key.
func (a *app) picture(doc string) (Picture, error) {
	if doc == "" {
		return Picture{}, errNoDoc
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	bucket := a.bucketOf(doc)
	pics, err := a.install.Pictures(ctx, bucket, a.rootFolder(ctx, bucket))
	if err != nil {
		return Picture{}, err
	}
	for _, p := range pics {
		if p.Doc == doc {
			p.Bucket = bucket
			return p, nil
		}
	}
	return Picture{}, errors.New("картинка не найдена: " + doc)
}

// publicURL is the address a person can give to somebody else.
func (a *app) publicURL(p Picture) string {
	if !IsPublic(p) || a.publicBaseNow() == "" {
		return ""
	}
	return strings.TrimRight(a.publicBaseNow(), "/") + "/" + p.Name
}

// hasArticles says whether the module has a long-form collection at all.
func (a *app) hasArticles() bool {
	return a.articles != nil && a.articlesBucket() != ""
}

// articlesList is the table of contents: the chapters, newest first.
func (a *app) articlesList(w http.ResponseWriter, r *http.Request) {
	if !a.hasArticles() {
		writeJSON(w, http.StatusOK, map[string]any{"articles": []Article{}, "note": "коллекция статей не настроена"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	list, err := a.articles.List(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	// A search over the chapters themselves: the title, the author, the date and the text. The module
	// already holds every chapter, so this costs nothing and returns a line to read before opening it
	// (23.11.2026).
	if query := strings.TrimSpace(r.URL.Query().Get("q")); query != "" {
		found := make([]Article, 0, len(list))
		for _, article := range list {
			if ok, snippet := matchArticle(article, query); ok {
				article.Snippet = snippet
				found = append(found, article)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"bucket": a.articlesBucket(), "articles": found, "query": query})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bucket": a.articlesBucket(), "articles": list})
}

// article answers one chapter as a page.
func (a *app) article(w http.ResponseWriter, r *http.Request) {
	if !a.hasArticles() {
		writeError(w, http.StatusNotFound, errNoArticles)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, errors.New("нужна статья (name)"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	page, err := a.articles.Article(ctx, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	// The screen resolves a picture of the article to its document key, so the article's pictures use
	// the same viewer as the grid.
	pics, err := a.pictures(ctx)
	if err == nil {
		byName := map[string]string{}
		for _, p := range pics {
			byName[p.Name] = p.Doc
		}
		docs := map[string]string{}
		for _, name := range page["pictures"].([]string) {
			if doc, ok := byName[name]; ok {
				docs[name] = doc
			}
		}
		page["docs"] = docs
		// The chapter's pictures are fetched through the relay as well: the permit travels with the
		// page (23.11.2026).
		if wantsMedia(r) {
			for _, doc := range docs {
				if media, ok := a.media(ctx, doc, 1400); ok {
					page["media"] = media
					break
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, page)
}

// errNoArticles says the module has no long-form collection configured.
var errNoArticles = errors.New("коллекция статей не настроена")

// saveArticle writes a chapter back: the editor shows markdown, and the installation writes the file
// and re-indexes it at once (doc.save_raw). Only a person with the write right may do it — the module
// trusts the header the installation puts on every request.
func (a *app) saveArticle(w http.ResponseWriter, r *http.Request) {
	if !mayWrite(r) {
		writeError(w, http.StatusForbidden, errors.New("эта роль не позволяет править статьи"))
		return
	}
	if !a.hasArticles() {
		writeError(w, http.StatusNotFound, errNoArticles)
		return
	}
	var in struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("тело запроса не разобрано: %w", err))
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, errors.New("нужна статья (name)"))
		return
	}
	// An empty text is a slip, not a decision: the chapter would stay, emptied, and nobody would
	// notice until the next read. Erasing a document is another button.
	if strings.TrimSpace(in.Text) == "" {
		writeError(w, http.StatusBadRequest, errors.New("пустой текст не сохраняется"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	// The document key comes from the chapter list: a name alone is not a key of the collection.
	list, err := a.articles.List(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	doc := ""
	for _, article := range list {
		if article.Name == in.Name {
			doc = article.Doc
			break
		}
	}
	if doc == "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("статья не найдена: %s", in.Name))
		return
	}
	if _, err := a.install.call(ctx, "doc.save_raw", map[string]any{
		"bucket": a.articlesBucket(), "doc": doc, "text": in.Text,
	}); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	a.articles.Forget()
	writeJSON(w, http.StatusOK, map[string]any{"name": in.Name, "doc": doc, "saved": true})
}

// mayWrite answers whether the person behind this request holds the write right. An empty header is the
// owner at the machine — the installation sends it with every request it proxies.
func mayWrite(r *http.Request) bool {
	rights := strings.TrimSpace(r.Header.Get("X-AnyText-Rights"))
	if rights == "" {
		return true
	}
	for _, right := range strings.Split(rights, ",") {
		if strings.EqualFold(strings.TrimSpace(right), "write") {
			return true
		}
	}
	return false
}

// describePicture asks the installation to describe a picture with its vision model — the same
// command the window's «Описать моделью» button runs (doc.describe). The answer is put into the
// editor's description field, not saved: a model's words are a draft a person reads first
// (23.11.2026).
func (a *app) describePicture(w http.ResponseWriter, r *http.Request) {
	if !mayWrite(r) {
		writeError(w, http.StatusForbidden, errors.New("эта роль не позволяет описывать картинки"))
		return
	}
	doc := strings.TrimSpace(r.URL.Query().Get("doc"))
	if doc == "" {
		writeError(w, http.StatusBadRequest, errNoDoc)
		return
	}
	// The installation gives a model two minutes for one picture; the request waits a little longer
	// than that, or a slow model would look like a broken module.
	ctx, cancel := context.WithTimeout(r.Context(), 150*time.Second)
	defer cancel()
	raw, err := a.install.call(ctx, "doc.describe", map[string]any{"bucket": a.bucketOf(doc), "doc": doc})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	var answer struct {
		Text  string `json:"text"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("ответ установки не разобран: %w", err))
		return
	}
	if strings.TrimSpace(answer.Text) == "" {
		writeError(w, http.StatusBadGateway, errors.New("модель не сказала ничего"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc, "text": answer.Text, "model": answer.Model})
}

// pictureEdges answers the graph relations of one picture — the «Связи» list the old gallery showed
// under a picture. Asked for the picture a person has just opened, and remembered for a couple of
// minutes, because walking the viewer with the arrows should not mean a call per step (23.11.2026).
func (a *app) pictureEdges(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, errors.New("нужно имя картинки (name)"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	list, err := a.Edges(ctx, name)
	if err != nil {
		// The endpoint draws a picture's connections on the screen. A memory that did not answer is a
		// missing list, not a broken gateway: answering 502 filled the browser console with red for
		// every unlinked picture, and «No edges found» is not an error at all (23.11.2026).
		writeJSON(w, http.StatusOK, map[string]any{
			"name": name, "edges": []Edge{}, "unavailable": true, "note": err.Error(),
		})
		return
	}
	if list == nil {
		list = []Edge{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "edges": list})
}

// decorate fills what every card needs: the public copy, the chapter the picture belongs to, the
// words a person wrote about it. One batch call for the memory, then a loop — the grid asks for this
// on every page and after every search (23.11.2026).
func (a *app) decorate(ctx context.Context, pics []Picture) {
	if a.memory != nil {
		names := make([]string, 0, len(pics))
		for _, p := range pics {
			names = append(names, p.Name)
		}
		if metas, err := a.memory.GetMany(ctx, names); err == nil {
			for i := range pics {
				if meta, ok := metas[pics[i].Name]; ok {
					pics[i].Meta = &meta
				}
			}
		}
	}
	for i := range pics {
		pics[i].Public = IsPublic(pics[i])
		if pics[i].Public {
			pics[i].PublicURL = a.publicURL(pics[i])
		}
		// Which chapter of the memoirs this picture is in: computed from the articles themselves, so
		// nobody has to fill a field by hand (22.11.2026).
		if a.articles != nil {
			if story, ok := a.articles.Story(ctx, pics[i].Name); ok {
				pics[i].Story, pics[i].StoryName = story.Title, story.Name
			}
		}
	}
	// The old gallery's own link: a picture whose meta names a chapter is shown with «Рассказ» even
	// when the chapter never spells the file name out. A chapter that no longer exists is not a link —
	// a button that leads nowhere is worse than no button (23.11.2026).
	if a.articles != nil {
		for i := range pics {
			if pics[i].StoryName != "" || pics[i].Meta == nil || strings.TrimSpace(pics[i].Meta.Memoir) == "" {
				continue
			}
			if article, ok := a.articles.ByName(ctx, strings.TrimSpace(pics[i].Meta.Memoir)); ok {
				pics[i].Story, pics[i].StoryName = article.Title, article.Name
			}
		}
	}
}

// pictureMatches is the local search of the grid: the file name, its folder, the description the
// installation holds and the gallery's own tags. Case-insensitive, and every word of the query has
// to be found — "верблюд балкон" finds a description that names both (23.11.2026).
func pictureMatches(p Picture, filter string) bool {
	haystack := strings.ToLower(p.Name + " " + p.Folder + " " + p.Description)
	if p.Meta != nil {
		haystack += " " + strings.ToLower(p.Meta.Description+" "+p.Meta.Tags+" "+p.Meta.Context)
	}
	for _, word := range strings.Fields(filter) {
		if !strings.Contains(haystack, word) {
			return false
		}
	}
	return true
}

// semanticSearch asks the installation's meaning search, which is what finds a picture by what is on
// it rather than by the words in its name. The answer is turned back into the shapes the screen already
// draws: card items for the pictures, chapter rows for the articles (23.11.2026).
func (a *app) semanticSearch(w http.ResponseWriter, r *http.Request) {
	in := r.URL.Query().Get("in")
	if in != "articles" {
		in = "pictures"
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, errors.New("пустой запрос"))
		return
	}
	bucket := a.bucket
	if in == "articles" {
		if !a.hasArticles() {
			writeError(w, http.StatusNotFound, errNoArticles)
			return
		}
		bucket = a.articlesBucket()
	}
	// The meaning search embeds the query and walks the vectors: a second and a half on this machine.
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	raw, err := a.install.call(ctx, "rag.search", map[string]any{"bucket": bucket, "query": query, "top_k": 60})
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	var answer struct {
		Results []searchHit `json:"results"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("ответ поиска не разобран: %w", err))
		return
	}
	answer.Results = trimHits(answer.Results)
	if in == "articles" {
		list, err := a.articles.List(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		byName := make(map[string]Article, len(list))
		for _, article := range list {
			byName[article.Name] = article
		}
		found := make([]Article, 0, len(answer.Results))
		for _, hit := range answer.Results {
			article, ok := byName[hit.Name]
			if !ok {
				continue
			}
			article.Snippet = plainSnippet(hit.Snippet)
			found = append(found, article)
		}
		writeJSON(w, http.StatusOK, map[string]any{"in": in, "query": query, "articles": found})
		return
	}

	pics, err := a.pictures(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	a.decorate(ctx, pics)
	byName := make(map[string]Picture, len(pics))
	for _, p := range pics {
		byName[p.Name] = p
	}
	items := make([]Picture, 0, len(answer.Results))
	for _, hit := range answer.Results {
		if p, ok := byName[hit.Name]; ok {
			items = append(items, p)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"in": in, "query": query, "items": items, "total": len(items)})
}

// searchHit is one answer of the installation's meaning search.
type searchHit struct {
	Doc     string  `json:"doc"`
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

// trimHits keeps the hits that are worth showing. The meaning search always answers with top_k
// documents, whether they are relevant or not, and its scores have no absolute scale: 0.53 for the
// pictures of this installation, 0.37 for its chapters. So the cut is relative to the best hit, with a
// ceiling and a floor — a search that returns the whole collection is not a search (23.11.2026).
func trimHits(hits []searchHit) []searchHit {
	const maxHits = 24
	const floor = 5
	if len(hits) == 0 {
		return hits
	}
	best := hits[0].Score
	for _, h := range hits {
		if h.Score > best {
			best = h.Score
		}
	}
	cutoff := best * 0.82
	keep := 0
	for keep < len(hits) && keep < maxHits && hits[keep].Score >= cutoff {
		keep++
	}
	if keep < floor {
		keep = floor
		if keep > len(hits) {
			keep = len(hits)
		}
	}
	return hits[:keep]
}

// signedMedia answers a fresh permit: the screen asks for one when a picture that used to load comes
// back 401, which is what an expired permit looks like (23.11.2026).
func (a *app) signedMedia(w http.ResponseWriter, r *http.Request) {
	doc := strings.TrimSpace(r.URL.Query().Get("doc"))
	max := 400
	if v := r.URL.Query().Get("max"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 2000 {
			max = n
		}
	}
	if doc == "" {
		writeError(w, http.StatusBadRequest, errNoDoc)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	// A refresh is asked for on purpose: the kept permit is what the screen already has.
	if a.mediaCache != nil {
		a.mediaCache.mu.Lock()
		a.mediaCache.token = ""
		a.mediaCache.mu.Unlock()
	}
	media, ok := a.media(ctx, doc, max)
	if !ok {
		// No relay, no account, or a relay that refused: the screen falls back to the plain address,
		// and this is not an error of the gateway — the console must stay clean (23.11.2026).
		writeJSON(w, http.StatusOK, map[string]any{"media": nil, "note": "пропуск не выписан"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"media": media})
}

// wantsMedia says whether the screen asked for the relay's permit with this request. The screen knows
// what the module cannot see: a page served as /s/<id>/api/module/... came through the relay, and its
// pictures need a permit; a page served at /api/module/... is on the machine that holds the pictures,
// and a detour through the relay there would only be slower (23.11.2026).
func wantsMedia(r *http.Request) bool {
	return r.URL.Query().Get("media") == "1" || r.Header.Get("X-AnyText-Relay") != ""
}

// publishArticle is the publication of one article: what the installation knows about it (GET),
// publishing it (POST), taking it down (DELETE). The module keeps no state of its own: the address,
// the date and the checksum of what went out live with the document, and the window that shows the
// article asks the same question (24.11.2026).
func (a *app) publishArticle(w http.ResponseWriter, r *http.Request) {
	doc := strings.TrimSpace(r.URL.Query().Get("doc"))
	if doc == "" {
		writeError(w, http.StatusBadRequest, errors.New("нужен документ (doc)"))
		return
	}
	// An article lives in the articles collection, not in the pictures one: the two are different
	// collections of the same installation, and asking the wrong one answers "not published" about
	// a chapter that is online (24.11.2026).
	bucket := a.bucketOf(doc)
	if b := a.articlesBucket(); b != "" && strings.HasPrefix(doc, b+"/") {
		bucket = b
	}
	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	var (
		raw json.RawMessage
		err error
	)
	switch r.Method {
	case http.MethodGet:
		raw, err = a.install.call(ctx, "doc.publication", map[string]any{"bucket": bucket, "doc": doc})
	case http.MethodPost:
		raw, err = a.install.call(ctx, "doc.publish", map[string]any{"bucket": bucket, "doc": doc})
	case http.MethodDelete:
		raw, err = a.install.call(ctx, "doc.unpublish", map[string]any{"bucket": bucket, "doc": doc})
	default:
		writeError(w, http.StatusMethodNotAllowed, errors.New("GET, POST or DELETE"))
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

// pictures answers the pictures of every chosen collection, each knowing where it came from. The flag
// "-bucket" is one collection, but a person may have several: generated pictures, screenshots,
// photographs, and the grid is the one place where they belong together (26.11.2026).
func (a *app) pictures(ctx context.Context) ([]Picture, error) {
	out := []Picture{}
	problems := []string{}
	for _, bucket := range a.pictureBuckets() {
		found, err := a.install.Pictures(ctx, bucket, a.rootFolder(ctx, bucket))
		if err != nil {
			problems = append(problems, bucket+": "+err.Error())
			continue
		}
		for i := range found {
			found[i].Bucket = bucket
		}
		out = append(out, found...)
	}
	if len(out) == 0 && len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	return out, nil
}

// ---------------------------------------------------------------- settings

// settingsAnswer is what the window needs to draw the form: the values in force and every collection
// of the installation with its folders, so a person chooses from a list instead of typing a name.
type settingsAnswer struct {
	Pictures     []string     `json:"pictures"`
	Articles     string       `json:"articles"`
	PublicBase   string       `json:"public_base"`
	Collections  []Collection `json:"collections"`
	SettingsFile string       `json:"settings_file,omitempty"`
}

// settingsGet answers the current settings and the collections of the installation.
func (a *app) settingsGet(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	answer := settingsAnswer{
		Pictures:     a.pictureBuckets(),
		Articles:     a.articlesBucket(),
		PublicBase:   a.publicBaseNow(),
		SettingsFile: a.setsPath,
	}
	if cols, err := a.install.Collections(ctx); err == nil {
		answer.Collections = cols
	}
	writeJSON(w, http.StatusOK, answer)
}

// settingsPost saves the choice: which collections of pictures the grid shows, which collection holds
// the chapters, and where the public copies live. It is applied at once — the screen reloads the grid
// and the articles reader starts reading the new collection (26.11.2026).
func (a *app) settingsPost(w http.ResponseWriter, r *http.Request) {
	if !mayWrite(r) {
		writeError(w, http.StatusForbidden, errors.New("эта роль не позволяет менять настройки"))
		return
	}
	var in settings
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("тело запроса не разобрано: %w", err))
		return
	}
	in = in.clean()
	if err := in.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cols, err := a.install.Collections(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	known := make(map[string]bool, len(cols))
	for _, c := range cols {
		known[c.ID] = true
	}
	for _, name := range in.Pictures {
		if !known[name] {
			writeError(w, http.StatusBadRequest, fmt.Errorf("нет коллекции «%s»", name))
			return
		}
	}
	if in.Articles != "" {
		if !known[in.Articles] {
			writeError(w, http.StatusBadRequest, fmt.Errorf("нет коллекции «%s»", in.Articles))
			return
		}
		// A collection of pictures as the warehouse of chapters would leave an empty tab with no word
		// about why: check for a chapter before saving.
		has, err := a.install.HasMarkdown(ctx, in.Articles)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		if !has {
			writeError(w, http.StatusBadRequest, fmt.Errorf("в коллекции «%s» нет ни одной статьи (.md)", in.Articles))
			return
		}
	}

	if err := saveSettings(a.setsPath, in); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	a.mu.Lock()
	a.sets = in
	a.bucket = in.Pictures[0]
	a.roots = map[string]rootEntry{}
	a.mu.Unlock()
	a.edges = newEdgesCache()
	a.mediaCache = nil

	writeJSON(w, http.StatusOK, map[string]any{
		"pictures": a.pictureBuckets(), "articles": a.articlesBucket(), "public_base": a.publicBaseNow(),
		"saved": true,
	})
}
