// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// config.go — what the owner chose in the window: the collections of pictures, the collection of
// articles and the public address of the copies.
//
// The flags are the defaults; this file overrides them. The module still keeps no database: this is
// the same command line, only editable. A person who has several collections of pictures marks them
// all, and a person who keeps chapters in «статьи» instead of «мемуары» says so once instead of
// asking for a reinstall (26.11.2026).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// settings is the file on disk.
type settings struct {
	// Pictures are the collections whose pictures the grid shows, in the order they were chosen.
	Pictures []string `json:"pictures"`
	// Articles is the collection of long-form chapters; empty means the module has no articles tab.
	Articles   string `json:"articles"`
	PublicBase string `json:"public_base"`
}

// settingsPath is where the choice lives: beside the other modules of this installation when the
// installation's own folder is known, and in the user's config folder otherwise.
func settingsPath() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "teonet", "anytext-gallery", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "teonet", "anytext-gallery", "config.json")
}

// clean returns the settings as they are stored: no empty names, no duplicates, no spaces.
func (s settings) clean() settings {
	out := settings{Articles: strings.TrimSpace(s.Articles), PublicBase: strings.TrimRight(strings.TrimSpace(s.PublicBase), "/")}
	seen := map[string]bool{}
	for _, name := range s.Pictures {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out.Pictures = append(out.Pictures, name)
	}
	return out
}

// validate says what is missing before anything is saved: a setting that names a collection nobody
// has is a setting that breaks the window after a restart.
func (s settings) validate() error {
	if len(s.Pictures) == 0 {
		return errors.New("выберите хотя бы одну коллекцию картинок")
	}
	return nil
}

// loadSettings reads the file; the second answer says whether the file was there at all, because
// "no file" means "use the flags" and not "an error".
func loadSettings(path string) (settings, bool) {
	if path == "" {
		return settings{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return settings{}, false
	}
	var s settings
	if err := json.Unmarshal(data, &s); err != nil {
		return settings{}, false
	}
	return s.clean(), true
}

// saveSettings writes the file atomically: a half-written config is a module that does not start.
func saveSettings(path string, s settings) error {
	if path == "" {
		return errors.New("некуда сохранять настройки: не найден домашний каталог")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(s.clean(), "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("настройки не сохранены: %w", err)
	}
	return nil
}
