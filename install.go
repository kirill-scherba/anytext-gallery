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
	// Vision says the installation described the picture with a model (the standard place for a
	// picture's description); HasDesc is the same fact for a screen that only needs a badge.
	Vision  bool `json:"vision"`
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

// Pictures reads the collection and answers with the images of its root folder, sorted by name: the
// generated files carry their time in the name, so the order is the history of the gallery.
func (c *installClient) Pictures(ctx context.Context, bucket, root string) ([]Picture, error) {
	raw, err := c.call(ctx, "rag.list", map[string]any{"bucket": bucket})
	if err != nil {
		return nil, err
	}
	var answer struct {
		Total   int `json:"total"`
		Results []struct {
			Doc            string `json:"doc"`
			Name           string `json:"name"`
			Folder         string `json:"folder"`
			Vision         bool   `json:"vision"`
			HasText        bool   `json:"has_text"`
			HasDescription bool   `json:"has_description"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return nil, fmt.Errorf("список документов не разобран: %w", err)
	}
	out := make([]Picture, 0, len(answer.Results))
	for _, d := range answer.Results {
		doc := strings.TrimSpace(d.Doc)
		if doc == "" || strings.HasPrefix(doc, "links/") {
			continue
		}
		if !imageExts[strings.ToLower(pathExt(doc))] {
			continue
		}
		if root != "" && d.Folder != root {
			continue
		}
		out = append(out, Picture{
			Doc:     doc,
			Name:    d.Name,
			Folder:  d.Folder,
			Vision:  d.Vision,
			HasDesc: d.HasDescription,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// pathExt is the lowercase extension of a document key.
func pathExt(doc string) string {
	if i := strings.LastIndex(doc, "."); i >= 0 {
		return doc[i:]
	}
	return ""
}
