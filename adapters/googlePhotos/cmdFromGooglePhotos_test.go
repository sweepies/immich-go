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

func TestExpandTakeoutArgsNonTakeoutZips(t *testing.T) {
	// Create a directory with non-takeout zip files
	tmpDir := t.TempDir()
	
	// Create a takeout zip file
	takeoutZip := filepath.Join(tmpDir, "takeout-001.zip")
	if err := os.WriteFile(takeoutZip, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Create a non-takeout zip file (should be excluded)
	otherZip := filepath.Join(tmpDir, "backup.zip")
	if err := os.WriteFile(otherZip, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	result, err := expandTakeoutArgs([]string{tmpDir})
	if err != nil {
		t.Fatalf("expandTakeoutArgs() error = %v", err)
	}
	
	// Should only return the takeout zip file
	if len(result) != 1 {
		t.Errorf("expandTakeoutArgs() got %d items, want 1 item", len(result))
		t.Errorf("got: %v", result)
		return
	}
	
	if result[0] != takeoutZip {
		t.Errorf("expandTakeoutArgs() = %v, want [%v]", result, takeoutZip)
	}
}

func TestExpandTakeoutArgsCaseInsensitive(t *testing.T) {
	// Create a directory with various case combinations
	tmpDir := t.TempDir()
	
	// Create takeout zip files with different cases
	zip1 := filepath.Join(tmpDir, "takeout-001.zip")
	zip2 := filepath.Join(tmpDir, "Takeout-002.ZIP")
	zip3 := filepath.Join(tmpDir, "TAKEOUT-003.zip")
	
	if err := os.WriteFile(zip1, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(zip2, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(zip3, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	result, err := expandTakeoutArgs([]string{tmpDir})
	if err != nil {
		t.Fatalf("expandTakeoutArgs() error = %v", err)
	}
	
	// Should return all three takeout zip files
	if len(result) != 3 {
		t.Errorf("expandTakeoutArgs() got %d items, want 3 items", len(result))
		t.Errorf("got: %v", result)
		return
	}
	
	// Use map-based comparison since order doesn't matter
	resultMap := make(map[string]bool)
	for _, r := range result {
		resultMap[r] = true
	}
	
	expected := []string{zip1, zip2, zip3}
	for _, exp := range expected {
		if !resultMap[exp] {
			t.Errorf("expandTakeoutArgs() missing expected item: %v", exp)
		}
	}
}
