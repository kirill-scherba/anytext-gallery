// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// memory.go — where the gallery's own words about a picture live.
//
// The description, the tags, the prompt and the link to a story sit in memory-store under
// memory/gallery/meta/<file name> — that is where the old gallery put them and where the index of this
// installation already keeps 151 of them. The module keeps writing there, and copies the description
// into the installation's standard place as well (see api.go): «не переносить, а копировать», because
// both galleries live side by side for a while (22.11.2026).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// metaKey is the memory key of one picture: the old gallery used the file name alone.
func metaKey(name string) string { return "memory/gallery/meta/" + name }

// memoryClient speaks MCP to memory-store through the installation's gateway.
type memoryClient struct {
	base   string
	server string
	http   *http.Client
}

func newMemoryClient(base, server string) *memoryClient {
	return &memoryClient{base: strings.TrimRight(base, "/"), server: server, http: &http.Client{Timeout: 60 * time.Second}}
}

// call runs one tool and answers with its structured value.
func (m *memoryClient) call(ctx context.Context, tool string, args map[string]any) (json.RawMessage, error) {
	if args == nil {
		args = map[string]any{}
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.base+"/"+m.server, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("память %s не ответила: %w", m.base, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	var answer struct {
		Result struct {
			Content           []struct{ Text string } `json:"content"`
			StructuredContent json.RawMessage         `json:"structuredContent"`
			IsError           bool                    `json:"isError"`
		} `json:"result"`
		Error *struct{ Message string } `json:"error"`
	}
	if err := json.Unmarshal(data, &answer); err != nil {
		return nil, fmt.Errorf("ответ памяти не разобран: %w", err)
	}
	if answer.Error != nil {
		return nil, fmt.Errorf("%s", answer.Error.Message)
	}
	if answer.Result.IsError {
		text := ""
		if len(answer.Result.Content) > 0 {
			text = answer.Result.Content[0].Text
		}
		return nil, fmt.Errorf("%s", text)
	}
	if len(answer.Result.StructuredContent) > 0 && string(answer.Result.StructuredContent) != "null" {
		return answer.Result.StructuredContent, nil
	}
	if len(answer.Result.Content) > 0 {
		return json.RawMessage(answer.Result.Content[0].Text), nil
	}
	return nil, fmt.Errorf("пустой ответ памяти на %s", tool)
}

// Meta is what a person writes about a picture.
type Meta struct {
	Description string `json:"description,omitempty"`
	Tags        string `json:"tags,omitempty"`
	Context     string `json:"context,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	Memoir      string `json:"memoir,omitempty"`
}

// Empty says there is nothing written about the picture.
func (m Meta) Empty() bool {
	return m.Description == "" && m.Tags == "" && m.Context == "" && m.Prompt == "" && m.Memoir == ""
}

// value is the record as memory-store keeps it.
func (m Meta) value() map[string]any {
	out := map[string]any{}
	if m.Description != "" {
		out["description"] = m.Description
	}
	if m.Tags != "" {
		out["tags"] = m.Tags
	}
	if m.Context != "" {
		out["context"] = m.Context
	}
	if m.Prompt != "" {
		out["prompt"] = m.Prompt
	}
	if m.Memoir != "" {
		out["memoir"] = m.Memoir
	}
	return out
}

// Get answers the meta of one picture, or an empty one.
func (m *memoryClient) Get(ctx context.Context, name string) (Meta, error) {
	batch, err := m.GetMany(ctx, []string{name})
	if err != nil {
		return Meta{}, err
	}
	return batch[name], nil
}

// GetMany answers the meta of several pictures in one call: a page of the grid asks once.
//
// memory_get answers in one of three shapes depending on how it is called: a pretty JSON record for
// one key, a list of records separated by \x01 for a batch, or an array. All three are read here -
// the tool has changed its clothes before, and a gallery must not depend on the shape of the day.
func (m *memoryClient) GetMany(ctx context.Context, names []string) (map[string]Meta, error) {
	out := make(map[string]Meta, len(names))
	if len(names) == 0 {
		return out, nil
	}
	keys := make([]string, 0, len(names))
	for _, n := range names {
		keys = append(keys, metaKey(n))
	}
	raw, err := m.call(ctx, "memory_get", map[string]any{"keys": keys})
	if err != nil {
		return nil, err
	}
	// The answer may already be plain text (the batch form) or a JSON string holding it.
	text := string(raw)
	var quoted string
	if err := json.Unmarshal(raw, &quoted); err == nil {
		text = quoted
	}
	if out = parseBatchText(text); len(out) > 0 {
		return out, nil
	}
	// An array of {key, value} records.
	var records []struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &records); err == nil && len(records) > 0 {
		for _, r := range records {
			out[nameOfKey(r.Key)] = metaFromValue(r.Value)
		}
		return out, nil
	}
	// An object keyed by key.
	var byKey map[string]json.RawMessage
	if err := json.Unmarshal(raw, &byKey); err == nil {
		for k, v := range byKey {
			out[nameOfKey(k)] = metaFromValue(v)
		}
	}
	return out, nil
}

// parseBatchText reads the batch answer: records separated by \x01, key then value.
func parseBatchText(text string) map[string]Meta {
	out := map[string]Meta{}
	if !strings.Contains(text, "\x01") {
		return out
	}
	parts := strings.Split(strings.TrimPrefix(text, "\x01"), "\x01")
	for i := 0; i+1 < len(parts); i += 2 {
		key := strings.TrimSpace(parts[i])
		if !strings.HasPrefix(key, "memory/gallery/meta/") {
			continue
		}
		out[nameOfKey(key)] = metaFromValue(json.RawMessage(parts[i+1]))
	}
	return out
}

// metaFromValue reads one record: either the meta itself, or a record with the meta in "content".
func metaFromValue(raw json.RawMessage) Meta {
	if len(raw) == 0 {
		return Meta{}
	}
	var record struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(raw, &record); err == nil && record.Content != "" {
		var meta Meta
		if json.Unmarshal([]byte(record.Content), &meta) == nil {
			return meta
		}
	}
	var meta Meta
	_ = json.Unmarshal(raw, &meta)
	return meta
}

// nameOfKey strips the prefix of a memory key.
func nameOfKey(key string) string {
	return strings.TrimPrefix(key, "memory/gallery/meta/")
}

// Save writes the meta of one picture.
func (m *memoryClient) Save(ctx context.Context, name string, meta Meta) error {
	value, err := json.Marshal(meta.value())
	if err != nil {
		return err
	}
	// The text of the record is what the embedding is built from: words a person would search for.
	text := strings.TrimSpace(meta.Description + " " + meta.Tags + " " + meta.Context)
	if text == "" {
		text = name
	}
	_, err = m.call(ctx, "memory_save", map[string]any{
		"key": metaKey(name), "value": string(value), "text": text,
	})
	return err
}

// Delete removes the meta of one picture.
func (m *memoryClient) Delete(ctx context.Context, name string) error {
	_, err := m.call(ctx, "memory_delete", map[string]any{"key": metaKey(name)})
	return err
}

// Names lists the pictures that have meta — the one-time migration walks this list.
func (m *memoryClient) Names(ctx context.Context) ([]string, error) {
	raw, err := m.call(ctx, "memory_list", map[string]any{"prefix": "memory/gallery/meta/"})
	if err != nil {
		return nil, err
	}
	// The answer is text with a human header ("Found 151 memories:") followed by keys, one per line;
	// structured answers carry an array.
	var structured struct {
		Keys []string `json:"keys"`
	}
	if json.Unmarshal(raw, &structured) == nil && len(structured.Keys) > 0 {
		out := make([]string, 0, len(structured.Keys))
		for _, k := range structured.Keys {
			if name := strings.TrimPrefix(k, "memory/gallery/meta/"); name != k {
				out = append(out, name)
			}
		}
		return out, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		text = string(raw)
	}
	out := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if name := strings.TrimPrefix(line, "memory/gallery/meta/"); name != line && name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

// Edge is one relation of the memory graph that touches a picture: "иллюстрация", "заказал",
// "упоминает" and the rest of the vocabulary.
type Edge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
	Date     string `json:"date,omitempty"`
	Source   string `json:"source,omitempty"`
}

// Edges answers the graph relations of one picture, as memory-store knows them. The old gallery showed
// them under a picture in its viewer ("🔗 Связи"), and this is the same list, asked on demand: one call
// per picture a person actually opens, not one per card of the grid (23.11.2026).
func (m *memoryClient) Edges(ctx context.Context, name string) ([]Edge, error) {
	raw, err := m.call(ctx, "graph_get_edges", map[string]any{"entity": name})
	if err != nil {
		return nil, err
	}
	var edges []Edge
	if err := json.Unmarshal(raw, &edges); err == nil {
		return edges, nil
	}
	text := string(raw)
	// "No edges found for "x.png"" is an answer, not a failure: a picture nobody has linked yet has no
	// relations, and that is an empty list (23.11.2026 — the console filled with 502 for this).
	if strings.Contains(strings.ToLower(text), "no edges") {
		return []Edge{}, nil
	}
	// The tool answers with a sentence and the array under it: "Edges for "x.png": 2\n[...]".
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		if err := json.Unmarshal([]byte(strings.TrimSpace(text[i+1:])), &edges); err == nil {
			return edges, nil
		}
	}
	return nil, fmt.Errorf("ответ памяти о связях не разобран: %.120s", text)
}

// edgesCache keeps what the graph said about a picture for a couple of minutes: a person walks the
// viewer with the arrows, and every step must not be a call to memory-store.
type edgesCache struct {
	mu   sync.Mutex
	at   map[string]time.Time
	list map[string][]Edge
}

func newEdgesCache() *edgesCache {
	return &edgesCache{at: map[string]time.Time{}, list: map[string][]Edge{}}
}

// Edges answers the cached relations of a picture, asking memory-store when it has nothing fresh.
func (a *app) Edges(ctx context.Context, name string) ([]Edge, error) {
	if a.memory == nil {
		return nil, fmt.Errorf("память не настроена")
	}
	if a.edges == nil {
		a.edges = newEdgesCache()
	}
	a.edges.mu.Lock()
	if list, ok := a.edges.list[name]; ok && time.Since(a.edges.at[name]) < 2*time.Minute {
		a.edges.mu.Unlock()
		return list, nil
	}
	a.edges.mu.Unlock()

	list, err := a.memory.Edges(ctx, name)
	if err != nil {
		return nil, err
	}
	a.edges.mu.Lock()
	a.edges.list[name], a.edges.at[name] = list, time.Now()
	a.edges.mu.Unlock()
	return list, nil
}
