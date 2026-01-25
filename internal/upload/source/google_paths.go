package source

import (
	"os"
	"path/filepath"
	"strings"
)

func expandTakeoutArgs(args []string) ([]string, error) {
	expanded := make([]string, 0, len(args))
	seen := map[string]struct{}{}

	for _, arg := range args {
		clean := filepath.Clean(arg)
		cleanLower := strings.ToLower(clean)
		if _, ok := seen[cleanLower]; ok {
			continue
		}
		seen[cleanLower] = struct{}{}

		info, err := os.Stat(clean)
		if err != nil || !info.IsDir() {
			expanded = append(expanded, clean)
			continue
		}

		entries, err := os.ReadDir(clean)
		if err != nil {
			return nil, err
		}

		foundZips := false
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			nameLower := strings.ToLower(entry.Name())
			if !strings.HasPrefix(nameLower, "takeout-") || !strings.HasSuffix(nameLower, ".zip") {
				continue
			}
			zipPath := filepath.Clean(filepath.Join(clean, entry.Name()))
			zipLower := strings.ToLower(zipPath)
			if _, ok := seen[zipLower]; ok {
				continue
			}
			seen[zipLower] = struct{}{}
			expanded = append(expanded, zipPath)
			foundZips = true
		}

		if !foundZips {
			expanded = append(expanded, clean)
		}
	}

	return expanded, nil
}
