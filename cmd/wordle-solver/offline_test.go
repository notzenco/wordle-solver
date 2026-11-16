package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --offline plays only puzzles whose answers are already cached, and says
// clearly when one isn't, instead of reaching the network.
func TestOfflineUsesOnlyCachedAnswers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("WORDLE_SOLVER_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "answers.json"), []byte(`{"1000":"crane"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"auto", "--offline", "--id", "1000", "--by", "solver"},
		{"auto", "--offline", "--id", "1000", "--dry"},
		{"assist", "--offline", "--id", "1000", "--by", "solver"},
	} {
		code, out, errOut := runWith(t, "crane\n", args...)
		if code != 0 || !strings.Contains(out, "#1,000") {
			t.Errorf("%v: code %d\n%s\n%s", args, code, out, errOut)
		}
	}
	code, _, errOut := runWith(t, "", "auto", "--offline", "--id", "1001", "--by", "solver")
	if code == 0 || !strings.Contains(errOut, "isn't cached") || !strings.Contains(errOut, "sync") {
		t.Fatalf("uncached puzzle: code %d, %q", code, errOut)
	}
}

func TestHelpSaysTheDefaultFetchesAndSavesToday(t *testing.T) {
	_, out, _ := run(t, "help")
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "auto ") && strings.Contains(line, "default") {
			if !strings.Contains(line, "today") || !strings.Contains(line, "save") {
				t.Fatalf("auto's summary should say the default fetches and saves today's puzzle: %q", line)
			}
			return
		}
	}
	t.Fatalf("no auto line in help:\n%s", out)
}
