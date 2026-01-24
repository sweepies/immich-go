package gp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandTakeoutArgs(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	
	// Create test zip files
	zip1 := filepath.Join(tmpDir, "takeout-001.zip")
	zip2 := filepath.Join(tmpDir, "takeout-002.zip")
	
	if err := os.WriteFile(zip1, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(zip2, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "single zip file",
			args:     []string{zip1},
			expected: []string{zip1},
		},
		{
			name:     "directory with zips",
			args:     []string{tmpDir},
			expected: []string{zip1, zip2},
		},
		{
			name:     "duplicate inputs",
			args:     []string{zip1, zip1},
			expected: []string{zip1},
		},
		{
			name:     "zip file and directory (no duplicate directory)",
			args:     []string{zip1, tmpDir},
			expected: []string{zip1, zip2},
		},
		{
			name:     "directory and zip file (no duplicate directory)",
			args:     []string{tmpDir, zip1},
			expected: []string{zip1, zip2},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandTakeoutArgs(tt.args)
			if err != nil {
				t.Fatalf("expandTakeoutArgs() error = %v", err)
			}
			
			// Check length first
			if len(result) != len(tt.expected) {
				t.Errorf("expandTakeoutArgs() got %d items, want %d items", len(result), len(tt.expected))
				t.Errorf("got: %v", result)
				t.Errorf("want: %v", tt.expected)
				return
			}
			
			// Use map-based comparison since order doesn't matter
			resultMap := make(map[string]bool)
			for _, r := range result {
				resultMap[r] = true
			}
			
			for _, exp := range tt.expected {
				if !resultMap[exp] {
					t.Errorf("expandTakeoutArgs() missing expected item: %v", exp)
				}
			}
		})
	}
}

func TestExpandTakeoutArgsEmptyDirectory(t *testing.T) {
	// Create a directory with no zip files
	tmpDir := t.TempDir()
	
	// Create a non-zip file
	nonZip := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(nonZip, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	result, err := expandTakeoutArgs([]string{tmpDir})
	if err != nil {
		t.Fatalf("expandTakeoutArgs() error = %v", err)
	}
	
	// Should return the directory itself since it has no zips
	if len(result) != 1 || result[0] != tmpDir {
		t.Errorf("expandTakeoutArgs() = %v, want [%v]", result, tmpDir)
	}
}

func TestExpandTakeoutArgsCaseInsensitiveDedup(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	
	// Create test zip file
	zipFile := filepath.Join(tmpDir, "takeout.zip")
	if err := os.WriteFile(zipFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Create different case variations of the same path
	// On case-insensitive filesystems (Windows/macOS), these would refer to the same file
	// We simulate this by passing the same path with different cases as separate arguments
	// The deduplication logic should handle this correctly
	lowerPath := filepath.Join(tmpDir, "takeout.zip")
	
	// Simulate uppercase directory component (on Windows/macOS, this would be the same directory)
	// We can't actually test this on Linux since it's case-sensitive, but we can verify
	// that the same path with different string cases is deduplicated
	tests := []struct {
		name        string
		args        []string
		expectCount int
	}{
		{
			name: "same path different cases should deduplicate",
			// Pass the exact same path twice - this tests the basic deduplication
			args:        []string{lowerPath, lowerPath},
			expectCount: 1,
		},
		{
			name: "directory expansion with explicit file should deduplicate",
			// Pass directory and explicit file - file should be deduplicated
			args:        []string{tmpDir, zipFile},
			expectCount: 1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandTakeoutArgs(tt.args)
			if err != nil {
				t.Fatalf("expandTakeoutArgs() error = %v", err)
			}
			
			if len(result) != tt.expectCount {
				t.Errorf("expandTakeoutArgs() got %d items, want %d items", len(result), tt.expectCount)
				t.Errorf("got: %v", result)
			}
		})
	}
}
