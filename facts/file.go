// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
	"github.com/goccy/go-yaml"
)

func gatherFileFacts(ctx context.Context, opts model.FactsConfig, log model.Logger) (map[string]any, error) {
	facts := map[string]any{}
	for _, dir := range []string{opts.SystemConfigDirectory, opts.UserConfigDirectory} {
		if dir == "" {
			continue
		}

		if !filepath.IsAbs(dir) {
			log.Error("Skipping facts directory with relative path", "dir", dir)
			continue
		}

		dir = filepath.Clean(dir)

		facts = iu.DeepMergeMap(facts, readFactsFile(filepath.Join(dir, "facts.json"), json.Unmarshal, log))
		facts = iu.DeepMergeMap(facts, readFactsFile(filepath.Join(dir, "facts.yaml"), yaml.Unmarshal, log))
		facts = iu.DeepMergeMap(facts, readFactsDir(filepath.Join(dir, "facts.d"), log))
	}

	return facts, nil
}

func readFactsDir(dir string, log model.Logger) map[string]any {
	if iu.IsSymlink(dir) {
		log.Error("Skipping facts directory that is a symlink", "dir", dir)
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	facts := map[string]any{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(dir, entry.Name())

		if entry.Type()&os.ModeSymlink != 0 {
			log.Error("Skipping facts file that is a symlink", "file", path)
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))

		switch ext {
		case ".json":
			facts = iu.DeepMergeMap(facts, readFactsFile(path, json.Unmarshal, log))
		case ".yaml":
			facts = iu.DeepMergeMap(facts, readFactsFile(path, yaml.Unmarshal, log))
		default:
			log.Debug("Skipping non-facts file", "file", path)
		}
	}

	return facts
}

func readFactsFile(path string, unmarshal func([]byte, any) error, log model.Logger) map[string]any {
	if !iu.FileExists(path) {
		return nil
	}

	if iu.IsSymlink(path) {
		log.Error("Skipping facts file that is a symlink", "file", path)
		return nil
	}

	log.Debug("Reading facts", "file", path)

	jb, err := os.ReadFile(path)
	if err != nil {
		log.Error("Failed to read facts file", "file", path, "error", err)
		return nil
	}

	var f map[string]any
	err = unmarshal(jb, &f)
	if err != nil {
		log.Error("Failed to unmarshal facts file", "file", path, "error", err)
		return nil
	}

	return f
}
