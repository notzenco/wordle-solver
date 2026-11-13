package main

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// committedRuns reads the benchmark results the docs were generated from.
func committedRuns(t *testing.T) []run {
	t.Helper()
	b, err := os.ReadFile("../../docs/results.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Runs []run }
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Runs) != 10 {
		t.Fatalf("want 5 strategies in 2 modes, got %d runs", len(data.Runs))
	}
	return data.Runs
}

// readText reads a checked-out file, undoing the CRLF line endings Git may
// give it on Windows.
func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func section(t *testing.T, path, name string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<!-- generated:` + name + ` -->\n(.*?)<!-- /generated:` + name + ` -->`).FindStringSubmatch(readText(t, path))
	if m == nil {
		t.Fatalf("%s has no generated:%s section", path, name)
	}
	return m[1]
}

// The published tables and charts must be exactly what the committed data
// produces, so no number in the docs can drift from results.json.
func TestDocsMatchCommittedResults(t *testing.T) {
	runs := committedRuns(t)
	for name, want := range map[string]string{
		"results":  resultsTable(runs),
		"headline": headlineTable(runs),
		"openers":  openersTable(runs),
		"failures": failuresList(runs),
	} {
		if got := section(t, "../../docs/analysis.md", name); got != want {
			t.Errorf("docs/analysis.md %s section differs from results.json:\n got %q\nwant %q", name, got, want)
		}
	}
	if got := section(t, "../../README.md", "headline"); got != headlineTable(runs) {
		t.Errorf("README.md headline differs from results.json")
	}
	for file, want := range map[string]string{
		"distribution.svg": distributionChart(runs),
		"mean-guesses.svg": meanChart(runs),
		"runtime.svg":      runtimeChart(runs),
	} {
		if readText(t, "../../docs/img/"+file) != want {
			t.Errorf("docs/img/%s differs from what results.json produces", file)
		}
	}
}

func TestReplaceSection(t *testing.T) {
	doc := "a\n<!-- generated:x -->\nold\n<!-- /generated:x -->\nb\n"
	got := replaceSection(doc, "x", "new $1\n")
	if got != "a\n<!-- generated:x -->\nnew $1\n<!-- /generated:x -->\nb\n" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "old") {
		t.Fatal("old body kept")
	}
}
