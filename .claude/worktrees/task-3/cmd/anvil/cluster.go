package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"gopkg.in/yaml.v3"
)

type watchFrontmatter struct {
	Path      string    `yaml:"path"`
	WatchedAt time.Time `yaml:"watched_at"`
}

func projectHash(absPath string) string {
	h := sha256.Sum256([]byte(absPath))
	return fmt.Sprintf("%x", h[:4])
}

func loadAllWatched() ([]watchFrontmatter, error) {
	watchedDir := config.WatchedDir()
	dirs, err := os.ReadDir(watchedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []watchFrontmatter
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		dirPath := filepath.Join(watchedDir, d.Name())
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		// Sort entries, take the latest .md file
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() > entries[j].Name()
		})

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}

			data, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
			if err != nil {
				break
			}

			content := string(data)
			start := strings.Index(content, "---\n")
			if start == -1 {
				break
			}
			end := strings.Index(content[start+4:], "\n---")
			if end == -1 {
				break
			}

			var fm watchFrontmatter
			if err := yaml.Unmarshal([]byte(content[start+4:start+4+end]), &fm); err != nil {
				break
			}

			result = append(result, fm)
			break
		}
	}

	return result, nil
}
