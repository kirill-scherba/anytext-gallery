// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// articles.go — the long-form part of the gallery: the memoirs of the installation, which are
// markdown documents of a collection.
//
// The old gallery rendered them on the Perl server; here the module reads the text through the
// installation and renders it itself, and every picture inside an article opens the module's own
// viewer instead of leading to bmat.uk (22.11.2026).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
)

// articleCacheTTL is how long the list of articles (and the picture -> story index) is trusted.
const articleCacheTTL = 2 * time.Minute

// Article is one chapter: what the table of contents needs and what the page is built from.
type Article struct {
	Name string `json:"name"`
	// Doc is the document key: the editor needs it to write the file back.
	Doc         string   `json:"doc,omitempty"`
	Title       string   `json:"title"`
	Date        string   `json:"date,omitempty"`
	Place       string   `json:"place,omitempty"`
	Author      string   `json:"author,omitempty"`
	Description string   `json:"description,omitempty"`
	Pictures    []string `json:"pictures,omitempty"`
	// Snippet is the line a search found the query in: what a person needs to see before opening the
	// chapter (23.11.2026).
	Snippet string `json:"snippet,omitempty"`

	sortDate string
	text     string
	source   string
}

// articlesReader keeps the parsed articles and the picture index for a couple of minutes: a page of
// pictures asks "which story is this one in" sixty times, and re-reading fifty files each time is a
// waste of the installation's time.
type articlesReader struct {
	install *installClient
	bucket  string

	mu       sync.Mutex
	list     []Article
	byName   map[string]Article
	loadedAt time.Time
}

func newArticlesReader(install *installClient, bucket string) *articlesReader {
	return &articlesReader{install: install, bucket: bucket}
}

// markdown is the renderer: tables, lists, headings and images are enough for a memoir.
var markdown = goldmark.New()

var (
	// A picture in an article: ![alt](https://bmat.uk/i/name.png) or .../i/pub/name.png.
	reImageSrc = regexp.MustCompile(`src="[^"]*?/i/(?:pub/)?([^"/]+\.(?:png|jpe?g|gif|webp|bmp|svg|avif|tiff?))"`)
	// A link to a picture: [text](https://bmat.uk/i/name.png).
	reLinkHref = regexp.MustCompile(`href="[^"]*?/i/(?:pub/)?([^"/]+\.(?:png|jpe?g|gif|webp|bmp|svg|avif|tiff?))"`)
	// The same picture in markdown and as a bare address: ![alt](https://bmat.uk/i/x.png) and
	// "**Иллюстрация:** https://bmat.uk/i/x.png".
	reMDTarget  = regexp.MustCompile(`\]\(([^)\s]+?)\)`)
	reURLTarget = regexp.MustCompile(`(https?://[^\s)]+/i/(?:pub/)?[^\s)]+)`)
	// A picture is named by its file: with a host ("…/i/name.png") or without one ("name.png"), which
	// is what a chapter should carry — the address may change, the name may not (22.11.2026).
	rePicName = regexp.MustCompile(`(?:^|/i/(?:pub/)?|[/(])([^/\s)]+\.(?:png|jpe?g|gif|webp|bmp|svg|avif|tiff?))`)
	// A picture named without a host: src="name.png" and href="name.png".
	reBareImg  = regexp.MustCompile(`src="([^"/]+\.(?:png|jpe?g|gif|webp|bmp|svg|avif|tiff?))"`)
	reBareLink = regexp.MustCompile(`href="([^"/]+\.(?:png|jpe?g|gif|webp|bmp|svg|avif|tiff?))"`)
	// Markdown that a snippet should not carry: a picture becomes its caption, a link its words.
	reMDImage = regexp.MustCompile(`!\[([^]]*)\]\([^)]*\)`)
	reMDLink  = regexp.MustCompile(`\[([^]]*)\]\([^)]*\)`)
	reRule    = regexp.MustCompile(`(?m)^\s*(?:---+|\*\*\*+)\s*$`)
	// A rule that sits in the middle of a line of prose ("--- *— Барон*") is a rule as well.
	reDashes = regexp.MustCompile(`\s*-{3,}\s*`)
	// The **Key:** Value header of a chapter.
	reHeader = regexp.MustCompile(`^\*\*([^*]+):\*\*\s*(.*?)\s*$`)
	// A date as "12 июля 2026" or "2026-07-12".
	reISODate = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`)
	reDate    = regexp.MustCompile(`(\d{1,2})\s+([а-яё]+)\s+(\d{4})`)
)

var ruMonths = map[string]string{
	"января": "01", "февраля": "02", "марта": "03", "апреля": "04",
	"мая": "05", "июня": "06", "июля": "07", "августа": "08",
	"сентября": "09", "октября": "10", "ноября": "11", "декабря": "12",
}

// Articles answers the chapters, newest first, parsing them on the first call and then from memory.
func (a *app) Articles(ctx context.Context) ([]Article, error) {
	if a.articles == nil {
		return nil, fmt.Errorf("коллекция статей не настроена")
	}
	return a.articles.List(ctx)
}

// List answers the parsed chapters, newest first.
func (r *articlesReader) List(ctx context.Context) ([]Article, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.list) > 0 && time.Since(r.loadedAt) < articleCacheTTL {
		return r.list, nil
	}

	raw, err := r.install.call(ctx, "doc.list", map[string]any{"bucket": r.bucket, "limit": 10000})
	if err != nil {
		return nil, err
	}
	var answer struct {
		Documents []struct {
			Key  string `json:"key"`
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return nil, fmt.Errorf("список статей не разобран: %w", err)
	}

	list := make([]Article, 0, len(answer.Documents))
	byName := map[string]Article{}
	for _, d := range answer.Documents {
		if !strings.HasSuffix(strings.ToLower(d.Name), ".md") || strings.EqualFold(d.Name, "readme.md") {
			continue
		}
		text, err := r.text(ctx, d.Key)
		if err != nil {
			continue // one unreadable chapter must not hide the rest
		}
		article := parseArticle(d.Name, text)
		article.Doc = d.Key
		article.source = text
		list = append(list, article)
		for _, picture := range article.Pictures {
			// One picture may be in several chapters; the table shows the newest first, so the
			// first one met here wins.
			if _, seen := byName[picture]; !seen {
				byName[picture] = article
			}
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].sortDate != list[j].sortDate {
			return list[i].sortDate > list[j].sortDate
		}
		return list[i].Name < list[j].Name
	})

	r.list, r.byName, r.loadedAt = list, byName, time.Now()
	return r.list, nil
}

// matchArticle says whether the chapter answers the query, and where: in the title, the author, the
// date or the text. The snippet is cut around the first hit in the chapter itself.
func matchArticle(article Article, query string) (bool, string) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true, ""
	}
	haystacks := []string{article.Title, article.Author, article.Date, article.Description}
	for _, text := range haystacks {
		if strings.Contains(strings.ToLower(text), q) {
			return true, ""
		}
	}
	if i := strings.Index(strings.ToLower(article.text), q); i >= 0 {
		return true, snippetAround(article.text, i, len(q))
	}
	return false, ""
}

// snippetAround cuts a window of the text around a hit: the words just before and after it, on one
// line, with dots where the text was cut.
func snippetAround(text string, start, length int) string {
	const before, after = 90, 140
	from := start - before
	if from < 0 {
		from = 0
	}
	to := start + length + after
	if to > len(text) {
		to = len(text)
	}
	// Do not start or end in the middle of a word: a snippet is read, not parsed.
	for from > 0 && !isSpaceByte(text[from-1]) {
		from--
	}
	for to < len(text) && !isSpaceByte(text[to]) {
		to++
	}
	out := plainSnippet(text[from:to])
	if from > 0 {
		out = "… " + out
	}
	if to < len(text) {
		out = out + " …"
	}
	return out
}

// plainSnippet cleans a window of markdown for reading: a picture keeps its caption, a link its
// words, and the marks of the markup go away. A snippet is read, not rendered (23.11.2026).
func plainSnippet(text string) string {
	text = reMDImage.ReplaceAllString(text, "$1")
	text = reMDLink.ReplaceAllString(text, "$1")
	text = strings.NewReplacer("**", "", "*", "", "__", "", "`", "", "##", "", "#", "", ">", "").Replace(text)
	text = reRule.ReplaceAllString(text, " ")
	text = reDashes.ReplaceAllString(text, " ")
	return strings.Join(strings.Fields(text), " ")
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t' || b == '\r'
}

// ByName answers a chapter by its file name. The old gallery recorded «Иллюстрация к главе» in the
// picture's own meta, and that link must keep working even when the chapter does not spell the file
// name out — 19 such records came from bmat (23.11.2026).
func (r *articlesReader) ByName(ctx context.Context, name string) (Article, bool) {
	if _, err := r.List(ctx); err != nil {
		return Article{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, article := range r.list {
		if article.Name == name {
			return article, true
		}
	}
	return Article{}, false
}

// Forget drops the parsed articles: an edited chapter must be read from the file again.
func (r *articlesReader) Forget() {
	r.mu.Lock()
	r.list, r.byName, r.loadedAt = nil, nil, time.Time{}
	r.mu.Unlock()
}

// Story answers the chapter a picture is in, if any.
func (r *articlesReader) Story(ctx context.Context, picture string) (Article, bool) {
	if _, err := r.List(ctx); err != nil {
		return Article{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	article, ok := r.byName[picture]
	return article, ok
}

// text reads one article through the installation: the document's own text, markdown as written.
func (r *articlesReader) text(ctx context.Context, key string) (string, error) {
	// raw: the file itself, not its extracted form — markdown rendered from the extracted text loses
	// its blank lines, its lists and its rules (22.11.2026).
	raw, err := r.install.call(ctx, "doc.text", map[string]any{"bucket": r.bucket, "doc": key, "raw": true})
	if err != nil {
		return "", err
	}
	var answer struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return "", err
	}
	return answer.Text, nil
}

// Article answers one chapter as a page: the title, the date, and the body rendered to HTML with the
// pictures replaced by references the screen resolves to its own viewer.
func (r *articlesReader) Article(ctx context.Context, name string) (map[string]any, error) {
	list, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, article := range list {
		if article.Name != name {
			continue
		}
		html, err := r.render(ctx, article)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"name": article.Name, "doc": article.Doc, "title": article.Title, "date": article.Date,
			"place": article.Place, "author": article.Author, "html": html,
			"pictures": article.Pictures,
			// The source is what an editor shows: markdown as the file holds it.
			"source": article.source,
		}, nil
	}
	return nil, fmt.Errorf("статья не найдена: %s", name)
}

// render turns the markdown into a page, and every picture or link to a picture becomes a reference
// to the module's viewer: the article must not send a person to bmat.uk.
func (r *articlesReader) render(ctx context.Context, article Article) (string, error) {
	var buf bytes.Buffer
	if err := markdown.Convert([]byte(article.text), &buf); err != nil {
		return "", err
	}
	html := buf.String()
	html = reImageSrc.ReplaceAllString(html, `src="" data-gallery="$1"`)
	html = reLinkHref.ReplaceAllString(html, `href="#" data-gallery="$1"`)
	// ![alt](img_123.png): goldmark turns a bare name into src="img_123.png" — the name is what the
	// screen needs, and the address is none of its business.
	html = reBareImg.ReplaceAllString(html, `src="" data-gallery="$1"`)
	html = reBareLink.ReplaceAllString(html, `href="#" data-gallery="$1"`)
	return html, nil
}

// parseArticle reads the header the memoirs have always had: a "# Title", a few "**Key:** Value"
// lines, then the body. The rules are the old gallery's (gallery-api/server.pl), because the files
// were written for it.
func parseArticle(name, text string) Article {
	article := Article{Name: name, Title: strings.TrimSuffix(name, ".md")}
	rest := text

	// The first heading is the title.
	if i := strings.Index(rest, "\n"); strings.HasPrefix(rest, "# ") && i > 0 {
		article.Title = strings.TrimSpace(rest[2:i])
		rest = rest[i+1:]
	}
	rest = strings.TrimLeft(rest, "\n")

	// Header lines, until the first line that is not one.
	for {
		line, tail, ok := cutLine(rest)
		if !ok {
			break
		}
		m := reHeader.FindStringSubmatch(line)
		if m == nil {
			break
		}
		switch strings.TrimSpace(m[1]) {
		case "Автор":
			article.Author = m[2]
		case "Записано":
			// "**Записано:** Барон, 26 октября 2026, по рассказу Кирилла." — the author is the first
			// word, and the rest is a note.
			if article.Author == "" {
				if word, _, ok := strings.Cut(m[2], ","); ok {
					article.Author = strings.TrimSpace(word)
				} else {
					article.Author = strings.TrimSpace(m[2])
				}
			}
		case "Дата", "Дата события":
			article.Date = m[2]
		case "Место":
			article.Place = m[2]
		}
		rest = strings.TrimLeft(tail, "\n")
	}
	rest = strings.TrimLeft(strings.TrimPrefix(rest, "---"), "\n")

	article.text = rest
	article.Description = firstParagraph(rest)
	article.Pictures = picturesIn(text)
	article.sortDate = sortDateOf(name, article.Date)
	// The date a screen can format is ISO; the header may say "26 октября 2026" and the name may say
	// nothing at all. When the date is known, the ISO form wins (22.11.2026).
	if len(article.sortDate) == 8 {
		article.Date = article.sortDate[:4] + "-" + article.sortDate[4:6] + "-" + article.sortDate[6:]
	}
	return article
}

// cutLine splits the first line off; ok is false when there is no line left to read.
func cutLine(text string) (line, rest string, ok bool) {
	if text == "" {
		return "", "", false
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return strings.TrimRight(text[:i], "\r"), text[i+1:], true
	}
	return text, "", false
}

// firstParagraph is the first run of plain text: the line under a chapter title, for the table of
// contents.
func firstParagraph(body string) string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" && len(out) > 0 {
			break
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "*") ||
			strings.HasPrefix(line, "!") || strings.HasPrefix(line, "-") || line == "---" {
			continue
		}
		out = append(out, line)
	}
	text := strings.Join(out, " ")
	if runes := []rune(text); len(runes) > 220 {
		text = string(runes[:220]) + "…"
	}
	return text
}

// picturesIn lists the picture files an article mentions: in markdown, in HTML, and as a bare address
// in a header line (**Иллюстрация:** https://bmat.uk/i/...).
func picturesIn(text string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(u string) {
		if m := rePicName.FindStringSubmatch(u); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	for _, m := range reImageSrc.FindAllStringSubmatch(text, -1) {
		add("/i/" + m[1])
	}
	for _, m := range reLinkHref.FindAllStringSubmatch(text, -1) {
		add("/i/" + m[1])
	}
	for _, m := range reMDTarget.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	for _, m := range reURLTarget.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	return out
}

// sortDateOf gives a YYYYMMDD for the order: the file name first (the memoirs are named by date),
// then the written date, then nothing.
func sortDateOf(name, date string) string {
	if m := reISODate.FindStringSubmatch(name); m != nil {
		return m[1] + m[2] + m[3]
	}
	if m := reISODate.FindStringSubmatch(date); m != nil {
		return m[1] + m[2] + m[3]
	}
	if m := reDate.FindStringSubmatch(strings.ToLower(date)); m != nil {
		if month, ok := ruMonths[m[2]]; ok {
			day := m[1]
			if len(day) == 1 {
				day = "0" + day
			}
			return m[3] + month + day
		}
	}
	return ""
}
