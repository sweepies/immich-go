package source

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func takeoutDedupKey(path string) string {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(path)
	}
	return path
}

func expandTakeoutArgs(args []string) ([]string, error) {
	expanded := make([]string, 0, len(args))
	seen := map[string]struct{}{}

	for _, arg := range args {
		clean := filepath.Clean(arg)
		cleanKey := takeoutDedupKey(clean)
		if _, ok := seen[cleanKey]; ok {
			continue
		}
		seen[cleanKey] = struct{}{}

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
			zipKey := takeoutDedupKey(zipPath)
			if _, ok := seen[zipKey]; ok {
				continue
			}
			seen[zipKey] = struct{}{}
			expanded = append(expanded, zipPath)
			foundZips = true
		}

		if !foundZips {
			expanded = append(expanded, clean)
		}
	}

	return expanded, nil
}
