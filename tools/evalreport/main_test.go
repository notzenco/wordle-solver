package main

import (
	"os"
	"strings"
	"testing"
)

// The published eval table must be exactly what the committed transcripts
// produce, so no number in it can drift from the data.
func TestEvalDocMatchesCommittedTranscripts(t *testing.T) {
	want, err := build("../../docs/eval-results")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../../docs/eval.md")
	if err != nil {
		t.Fatal(err)
	}
	m := section.FindStringSubmatch(strings.ReplaceAll(string(b), "\r\n", "\n"))
	if m == nil {
		t.Fatal("docs/eval.md has no generated:eval-results section")
	}
	got := strings.TrimPrefix(m[0], m[1])
	got = strings.TrimSuffix(got, m[2])
	if got != want {
		t.Errorf("docs/eval.md results table differs from the transcripts:\n got:\n%s\nwant:\n%s", got, want)
	}
}
