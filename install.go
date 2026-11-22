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
	return &installClient{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 120 * time.Second}}
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
	Doc    string `json:"doc"`
	Name   string `json:"name"`
	Folder string `json:"folder,omitempty"`
	Size   int64  `json:"size,omitempty"`
	// HasDesc says the installation holds a description of this picture in its index (the standard
	// place); the gallery's own text lives in memory-store and is shown in the next step.
	HasDesc bool `json:"has_description"`
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
			Key    string `json:"key"`
			Name   string `json:"name"`
			Folder string `json:"folder"`
			Kind   string `json:"kind"`
			State  string `json:"state"`
			Size   int64  `json:"size"`
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
			Doc:     key,
			Name:    d.Name,
			Folder:  d.Folder,
			Size:    d.Size,
			HasDesc: d.HasDescription,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// pathExt is the lowercase extension of a document name.
func pathExt(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i:]
	}
	return ""
}
