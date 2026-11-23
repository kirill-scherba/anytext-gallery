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
	"time"
)

// app is the module with its two dependencies: the installation (files and pictures) and memory-store
// (the words about them).
type app struct {
	install *installClient
	memory  *memoryClient
	bucket  string
	// publicBase is the address the public copies are served at (the old gallery's /i/pub/).
	publicBase string
	// articles is the reader of the long-form collection (the memoirs), when one is configured.
	articles *articlesReader

	// root is the folder of the collection, read once: a gallery page must not ask the installation
	// for the collection list on every scroll step.
	root   string
	rootAt time.Time

	// edges is what the memory graph said about the pictures seen recently.
	edges *edgesCache
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
	mux.HandleFunc("GET /api/meta", a.meta)
	mux.HandleFunc("POST /api/describe", a.describePicture)
	mux.HandleFunc("GET /api/edges", a.pictureEdges)
	mux.HandleFunc("POST /api/meta", a.saveMeta)
	mux.HandleFunc("DELETE /api/meta", a.deleteMeta)
	mux.HandleFunc("POST /api/migrate", a.migrate)
	mux.HandleFunc("GET /api/public", a.publicState)
	mux.HandleFunc("POST /api/public", a.makePublic)
	mux.HandleFunc("DELETE /api/public", a.removePublic)
	mux.HandleFunc("POST /api/trash", a.trash)
	mux.HandleFunc("GET /api/articles", a.articlesList)
	mux.HandleFunc("GET /api/article", a.article)
	mux.HandleFunc("POST /api/article", a.saveArticle)
	// The screen is one document: its styles and its script come inside it (ui.go explains why).
	mux.HandleFunc("GET /ui/", a.page)
	return mux
}

// errNoDoc is the one missing argument of the meta endpoints.
var errNoDoc = errors.New("нужен документ (doc)")

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
	mode := q.Get("sort")
	if mode != "name" {
		mode = "new"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	pics, err := a.install.Pictures(ctx, a.bucket, a.rootFolder(ctx))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	sortPictures(pics, mode)
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
	page := pics[offset:end]
	for i := range page {
		page[i].Public = IsPublic(page[i])
		if page[i].Public {
			page[i].PublicURL = a.publicURL(page[i])
		}
		// Which chapter of the memoirs this picture is in: computed from the articles themselves, so
		// nobody has to fill a field by hand (22.11.2026).
		if a.articles != nil {
			if story, ok := a.articles.Story(ctx, page[i].Name); ok {
				page[i].Story = story.Title
				page[i].StoryName = story.Name
			}
		}
	}
	// The words about the pictures: one batch call for the page, not one per card.
	if a.memory != nil {
		names := make([]string, 0, len(page))
		for _, p := range page {
			names = append(names, p.Name)
		}
		if metas, err := a.memory.GetMany(ctx, names); err == nil {
			for i := range page {
				if meta, ok := metas[page[i].Name]; ok {
					page[i].Meta = &meta
				}
			}
		}
	}
	// The old gallery's own link: a picture whose meta names a chapter is shown with «Рассказ» even
	// when the chapter never spells the file name out. A chapter that no longer exists is not a link —
	// a button that leads nowhere is worse than no button (23.11.2026).
	if a.articles != nil {
		for i := range page {
			if page[i].StoryName != "" || page[i].Meta == nil || strings.TrimSpace(page[i].Meta.Memoir) == "" {
				continue
			}
			if article, ok := a.articles.ByName(ctx, strings.TrimSpace(page[i].Meta.Memoir)); ok {
				page[i].Story, page[i].StoryName = article.Title, article.Name
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket": a.bucket,
		"total":  total,
		"offset": offset,
		"limit":  limit,
		"items":  page,
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
			"bucket": a.bucket, "doc": doc, "text": meta.Description, "model": "gallery",
		})
		if err != nil {
			answer["index"] = err.Error()
		} else {
			answer["index"] = "ok"
		}
	} else {
		if _, err := a.install.call(ctx, "doc.drop_description", map[string]any{"bucket": a.bucket, "doc": doc}); err != nil {
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
	docs, err := a.install.Pictures(ctx, a.bucket, a.rootFolder(ctx))
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
			"bucket": a.bucket, "doc": doc, "text": meta.Description, "model": "gallery",
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
		"bucket": a.bucket, "doc": doc, "delete_file": false,
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
	pics, err := a.install.Pictures(ctx, a.bucket, a.rootFolder(ctx))
	if err != nil {
		return Picture{}, err
	}
	for _, p := range pics {
		if p.Doc == doc {
			return p, nil
		}
	}
	return Picture{}, errors.New("картинка не найдена: " + doc)
}

// publicURL is the address a person can give to somebody else.
func (a *app) publicURL(p Picture) string {
	if !IsPublic(p) || a.publicBase == "" {
		return ""
	}
	return strings.TrimRight(a.publicBase, "/") + "/" + p.Name
}

// articlesList is the table of contents: the chapters, newest first.
func (a *app) articlesList(w http.ResponseWriter, r *http.Request) {
	if a.articles == nil {
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
	writeJSON(w, http.StatusOK, map[string]any{"bucket": a.articles.bucket, "articles": list})
}

// article answers one chapter as a page.
func (a *app) article(w http.ResponseWriter, r *http.Request) {
	if a.articles == nil {
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
	pics, err := a.install.Pictures(ctx, a.bucket, a.rootFolder(ctx))
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
	if a.articles == nil {
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
		"bucket": a.articles.bucket, "doc": doc, "text": in.Text,
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
	raw, err := a.install.call(ctx, "doc.describe", map[string]any{"bucket": a.bucket, "doc": doc})
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
		writeError(w, http.StatusBadGateway, err)
		return
	}
	if list == nil {
		list = []Edge{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "edges": list})
}
