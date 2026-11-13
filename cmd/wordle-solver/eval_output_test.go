package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestEvalSummary(t *testing.T) {
	var out bytes.Buffer
	writeEvalSummary(&out, evalSummary{Model: "clef-flash-4bit", Kind: "systemone", Options: "candidates", Out: "clef.jsonl", Games: 3, Eligible: 3, Solved: 3, Guesses: 11, Beyond: 1, Tokens: 12328, Duration: 4302846250 * time.Nanosecond, MetricCount: 6, Probability: 1.6606, Loss: 12.8316, Uniform: 16.7532, Frequency: 16.3512})
	s := out.String()
	for _, want := range []string{"3 / 3 eligible games", "11 total, 3.67 per game", "+1 total guesses", "12,328", "4.303s", "27.7%", "2.139", "Transcript: clef.jsonl"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "map[") || strings.Contains(s, "\t") {
		t.Fatal("unformatted output", s)
	}
	for _, line := range strings.Split(s, "\n") {
		if len(line) > 78 {
			t.Fatal("wide output", line)
		}
	}
	t.Log("\n" + s)
}
func TestEvalSummaryNoObservations(t *testing.T) {
	var out bytes.Buffer
	writeEvalSummary(&out, evalSummary{Kind: "systemone", Distractors: 4})
	s := out.String()
	for _, want := range []string{"n/a (no eligible games)", "n/a (answer never offered)", "n/a (no valid choices)"} {
		if !strings.Contains(s, want) {
			t.Fatal(s)
		}
	}
	if strings.Contains(s, "NaN") || strings.Contains(s, "Inf") {
		t.Fatal(s)
	}
}
func TestChatSummary(t *testing.T) {
	var out bytes.Buffer
	writeEvalSummary(&out, evalSummary{Kind: "chat", Options: "free"})
	if strings.Contains(out.String(), "Decision scores") || strings.Contains(out.String(), "Input tokens") {
		t.Fatal(out.String())
	}
}
