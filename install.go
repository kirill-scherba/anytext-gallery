// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// install.go — the module's only dependency: the installation it belongs to.
//
// The pictures are documents of a collection, the previews and files are the installation's own
// endpoints, and the answer to "what is in the collection" is a command. The module keeps no list of
// its own: one truth, one place (docs/MODULES.md in anytext-gui).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// installClient speaks to the installation over its command channel on loopback.
type installClient struct {
	base string
	http *http.Client
}

func newInstallClient(base string) *installClient {
	// Three minutes: the installation gives a vision model two for one picture, and a call that waited
	// exactly as long as the model would look like a broken module (23.11.2026). Every other call
	// carries its own, shorter deadline.
	return &installClient{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 180 * time.Second}}
}

// call runs one command and answers with its result.
func (c *installClient) call(ctx context.Context, command string, args map[string]any) (json.RawMessage, error) {
	if args == nil {
		args = map[string]any{}
	}
	body, err := json.Marshal(map[string]any{"command": command, "args": args})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/command", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("установка %s не ответила: %w", c.base, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	var answer struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(data, &answer); err != nil {
		return nil, fmt.Errorf("ответ установки не разобран: %w", err)
	}
	if answer.Error != "" {
		return nil, fmt.Errorf("%s", answer.Error)
	}
	return answer.Result, nil
}

// Picture is one image of the collection, as the screen needs it.
type Picture struct {
	// Bucket is the collection this picture came from: the grid may show several, and a picture is
	// published, described and thrown away in its own collection (26.11.2026).
	Bucket string `json:"bucket,omitempty"`
	Doc    string `json:"doc"`
	Name   string `json:"name"`
	Folder string `json:"folder,omitempty"`
	Size   int64  `json:"size,omitempty"`
	// Path is the file on disk: the module needs it to place the public copy beside the picture.
	Path string `json:"path,omitempty"`
	// Public says a copy of the picture lies in the public folder next to it; PublicURL is the
	// address a person can give away.
	Public    bool   `json:"public,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
	// ModTime is when the file was last changed: the grid shows the newest first, because a name
	// does not say when a picture appeared (22.11.2026).
	ModTime string `json:"mod_time,omitempty"`
	// Story is the chapter of the memoirs this picture appears in, if any.
	Story     string `json:"story,omitempty"`
	StoryName string `json:"story_name,omitempty"`
	// Description is the text the installation holds about the picture - the standard place, where
	// "Describe with a model" and this gallery both write. Meta is the gallery's own record, kept for
	// the old gallery while it lives.
	Description string `json:"description,omitempty"`
	HasDesc     bool   `json:"has_description"`
	Meta        *Meta  `json:"meta,omitempty"`
}

// imageExts are the files a gallery shows. Everything else in the collection (markdown, audio,
// scripts) is a document, not a picture.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".bmp": true, ".svg": true, ".avif": true, ".tif": true, ".tiff": true,
}

// BucketRoot answers the first folder of a collection: the gallery shows the files of the root, not
// the copies in pub/ and not the pictures moved to the trash. The old gallery worked the same way -
// it listed the image directory without descending (22.11.2026).
func (c *installClient) BucketRoot(ctx context.Context, bucket string) (string, error) {
	raw, err := c.call(ctx, "bucket.list", nil)
	if err != nil {
		return "", err
	}
	var list []struct {
		ID      string   `json:"id"`
		Folders []string `json:"folders"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return "", fmt.Errorf("список коллекций не разобран: %w", err)
	}
	for _, b := range list {
		if b.ID == bucket && len(b.Folders) > 0 {
			return b.Folders[0], nil
		}
	}
	return "", fmt.Errorf("коллекция %q не найдена", bucket)
}

// Pictures reads the files of the collection and answers with its images, sorted by name: the
// generated files carry their time in the name, so the order is the history of the gallery.
//
// It asks for the FILES, not for the index: a picture without a description has no text, the scan
// skips it as "no_text", and it never becomes a document. The old gallery listed a directory and
// that was right; doc.list is the same list, with the keys the picture endpoints want (22.11.2026).
func (c *installClient) Pictures(ctx context.Context, bucket, root string) ([]Picture, error) {
	raw, err := c.call(ctx, "doc.list", map[string]any{
		"bucket": bucket,
		// Everything in one answer: a gallery grid pages in the browser, and a hundred documents
		// per request would make the counter lie about the collection.
		"limit": 10000,
	})
	if err != nil {
		return nil, err
	}
	var answer struct {
		Documents []struct {
			Key         string `json:"key"`
			Name        string `json:"name"`
			Folder      string `json:"folder"`
			Kind        string `json:"kind"`
			State       string `json:"state"`
			Size        int64  `json:"size"`
			Path        string `json:"path"`
			ModTime     string `json:"mod_time"`
			Description string `json:"description"`
			// A picture that has a description in the index says so; a person sees which ones do.
			HasDescription bool `json:"has_description"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return nil, fmt.Errorf("список файлов не разобран: %w", err)
	}
	out := make([]Picture, 0, len(answer.Documents))
	for _, d := range answer.Documents {
		key := strings.TrimSpace(d.Key)
		if key == "" || d.State == "excluded" {
			continue
		}
		if !strings.EqualFold(d.Kind, "image") && !imageExts[strings.ToLower(pathExt(d.Name))] {
			continue
		}
		if root != "" && filepath.Clean(d.Folder) != filepath.Clean(root) {
			continue
		}
		out = append(out, Picture{
			Doc:         key,
			Name:        d.Name,
			Folder:      d.Folder,
			Size:        d.Size,
			Path:        d.Path,
			ModTime:     d.ModTime,
			Description: d.Description,
			HasDesc:     d.HasDescription,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// publicPath is where the public copy of a picture lies: the pub folder beside it, as the old gallery
// had it. The folder is not part of the collection's root, so the grid does not list the copies.
func publicPath(p Picture) string {
	if p.Path == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(p.Path), "pub", p.Name)
}

// MakePublic copies the picture next to itself in pub/ and answers the file it wrote.
func MakePublic(p Picture) (string, error) {
	dst := publicPath(p)
	if dst == "" {
		return "", fmt.Errorf("у картинки нет пути на диске")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	in, err := os.Open(p.Path)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	_ = os.Chmod(dst, 0o644)
	return dst, nil
}

// RemovePublic deletes the public copy, if it exists.
func RemovePublic(p Picture) error {
	dst := publicPath(p)
	if dst == "" {
		return fmt.Errorf("у картинки нет пути на диске")
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsPublic says whether the public copy exists.
func IsPublic(p Picture) bool {
	dst := publicPath(p)
	if dst == "" {
		return false
	}
	_, err := os.Stat(dst)
	return err == nil
}

// sortPictures orders the grid: the newest first (what a gallery wants), or by name.
func sortPictures(pics []Picture, mode string) {
	if mode == "name" {
		sort.SliceStable(pics, func(i, j int) bool { return pics[i].Name < pics[j].Name })
		return
	}
	sort.SliceStable(pics, func(i, j int) bool {
		if pics[i].ModTime != pics[j].ModTime {
			return pics[i].ModTime > pics[j].ModTime // RFC 3339 sorts as text
		}
		return pics[i].Name < pics[j].Name
	})
}

// pathExt is the lowercase extension of a document name.
func pathExt(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i:]
	}
	return ""
}

// Collection is one collection of the installation as the settings form needs it: a name a person
// recognises and the folders behind it, which are the installation's business but are shown so that
// nobody has to guess where the pictures will come from (26.11.2026).
type Collection struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Folders []string `json:"folders"`
}

// Collections lists the collections of the installation.
func (c *installClient) Collections(ctx context.Context) ([]Collection, error) {
	raw, err := c.call(ctx, "bucket.list", nil)
	if err != nil {
		return nil, err
	}
	var out []Collection
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("список коллекций не разобран: %w", err)
	}
	return out, nil
}

// HasMarkdown says whether a collection holds at least one chapter: a person may point the articles
// section at a collection of pictures by mistake, and the module should say so before it saves.
func (c *installClient) HasMarkdown(ctx context.Context, bucket string) (bool, error) {
	raw, err := c.call(ctx, "doc.list", map[string]any{"bucket": bucket, "limit": 200})
	if err != nil {
		return false, err
	}
	var answer struct {
		Documents []struct {
			Name string `json:"name"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return false, fmt.Errorf("список документов не разобран: %w", err)
	}
	for _, d := range answer.Documents {
		if strings.HasSuffix(strings.ToLower(d.Name), ".md") && !strings.EqualFold(d.Name, "readme.md") {
			return true, nil
		}
	}
	return false, nil
}
