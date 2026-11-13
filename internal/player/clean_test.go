package player

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClean(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	os.MkdirAll(dir, 0o755)
	for _, p := range []string{"profile.json", "history.json", "answers.json", "answers.json.tmp", "notes.txt"} {
		os.WriteFile(filepath.Join(dir, p), []byte("{}"), 0o644)
	}

	removed, err := Clean(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 4 {
		t.Errorf("removed %d files, want 4: %v", len(removed), removed)
	}
	// A file the solver did not write is left alone, and so is its folder.
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Errorf("notes.txt was removed: %v", err)
	}

	os.Remove(filepath.Join(dir, "notes.txt"))
	os.WriteFile(filepath.Join(dir, "history.json"), []byte("{}"), 0o644)
	if _, err := Clean(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("empty data folder still exists: %v", err)
	}
	if removed, err := Clean(dir); err != nil || len(removed) != 0 {
		t.Errorf("cleaning nothing: removed %v, err %v", removed, err)
	}
}
