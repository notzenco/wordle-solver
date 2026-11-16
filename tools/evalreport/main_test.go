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

func TestTableOrdersSetsThenModels(t *testing.T) {
	rows := []row{
		{Model: "clef-8bit", Set: "matched", Eligible: 1},
		{Model: "clef-flash-4bit", Set: "heldout", Eligible: 1},
		{Model: "zz", Set: "answers", Eligible: 1},
		{Model: "clef-flash-4bit", Set: "answers", Eligible: 1},
		{Model: "clef-flash-4bit", Set: "matched", Eligible: 1},
	}
	var order []string
	for _, line := range strings.Split(table(rows), "\n")[2:] {
		if f := strings.Split(line, " | "); len(f) > 1 {
			order = append(order, strings.TrimPrefix(f[0], "| ")+"/"+f[1])
		}
	}
	want := "Clef Flash, 4-bit/answers zz/answers Clef Flash, 4-bit/held-out Clef Flash, 4-bit/matched Clef, 8-bit/matched"
	if got := strings.Join(order, " "); got != want {
		t.Fatalf("order:\n got %s\nwant %s", got, want)
	}
}

// A chat run has no probabilities, so its decision columns are blank, not 0.
func TestChatRowsLeaveDecisionScoresBlank(t *testing.T) {
	line := strings.Split(table([]row{{Model: "m", Set: "answers", Eligible: 10, Guesses: 60, Baseline: 40}}), "\n")[2]
	if strings.Contains(line, "0.000") || strings.Contains(line, "0.0%") || strings.Count(line, "| – ") != 4 {
		t.Fatalf("chat row: %s", line)
	}
}
