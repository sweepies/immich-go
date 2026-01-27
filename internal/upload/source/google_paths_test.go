package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandTakeoutArgs(t *testing.T) {
	tmpDir := t.TempDir()

	zip1 := filepath.Join(tmpDir, "takeout-001.zip")
	zip2 := filepath.Join(tmpDir, "takeout-002.zip")
	other := filepath.Join(tmpDir, "notes.txt")

	for _, p := range []string{zip1, zip2, other} {
		if err := os.WriteFile(p, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
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
			name:     "zip file and directory",
			args:     []string{zip1, tmpDir},
			expected: []string{zip1, zip2},
		},
		{
			name:     "directory and zip file",
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

			if len(result) != len(tt.expected) {
				t.Errorf("expandTakeoutArgs() got %d items, want %d items", len(result), len(tt.expected))
				t.Errorf("got: %v", result)
				t.Errorf("want: %v", tt.expected)
				return
			}

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
	tmpDir := t.TempDir()
	nonZip := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(nonZip, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := expandTakeoutArgs([]string{tmpDir})
	if err != nil {
		t.Fatalf("expandTakeoutArgs() error = %v", err)
	}

	if len(result) != 1 || result[0] != tmpDir {
		t.Errorf("expandTakeoutArgs() = %v, want [%v]", result, tmpDir)
	}
}

func TestExpandTakeoutArgsNonTakeoutZips(t *testing.T) {
	tmpDir := t.TempDir()

	takeoutZip := filepath.Join(tmpDir, "takeout-001.zip")
	if err := os.WriteFile(takeoutZip, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	otherZip := filepath.Join(tmpDir, "backup.zip")
	if err := os.WriteFile(otherZip, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := expandTakeoutArgs([]string{tmpDir})
	if err != nil {
		t.Fatalf("expandTakeoutArgs() error = %v", err)
	}

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
	tmpDir := t.TempDir()

	zip1 := filepath.Join(tmpDir, "takeout-001.zip")
	zip2 := filepath.Join(tmpDir, "Takeout-002.ZIP")
	zip3 := filepath.Join(tmpDir, "TAKEOUT-003.zip")

	for _, p := range []string{zip1, zip2, zip3} {
		if err := os.WriteFile(p, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	result, err := expandTakeoutArgs([]string{tmpDir})
	if err != nil {
		t.Fatalf("expandTakeoutArgs() error = %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expandTakeoutArgs() got %d items, want 3 items", len(result))
		t.Errorf("got: %v", result)
		return
	}

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

func TestTakeoutNameExtraction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"zip part", "takeout-20240101T120000Z-001.zip", "takeout-20240101T120000Z"},
		{"zip without part", "takeout-20240101T120000Z.zip", "takeout-20240101T120000Z"},
		{"folder name", "takeout-20240101T120000Z", "takeout-20240101T120000Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTakeoutName(tt.input)
			if got != tt.expected {
				t.Errorf("takeout name = %q, want %q", got, tt.expected)
			}
		})
	}
}
