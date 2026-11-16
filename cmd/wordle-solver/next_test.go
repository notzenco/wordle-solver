package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNextSuggestsFromTypedClues(t *testing.T) {
	code, out, errOut := run(t, "next", "slate:bbbbb", "--top", "3")
	if code != 0 || !strings.Contains(out, "listed answers fit") || strings.Count(out, "\n  ") < 3 {
		t.Fatalf("code %d\n%s\n%s", code, out, errOut)
	}
	code, out, _ = run(t, "next")
	if code != 0 || !strings.Contains(out, "RAISE") {
		t.Fatalf("no clues should give the opening: %d\n%s", code, out)
	}
	code, out, _ = run(t, "next", "crane:ggggg")
	if code != 0 || !strings.Contains(out, "Solved") {
		t.Fatalf("solved: %d\n%s", code, out)
	}
	var v struct {
		Candidates []string
		Guesses    []struct {
			Word     string
			Bits     float64
			Expected float64
			Worst    int
			Fits     bool
		}
	}
	code, out, _ = run(t, "next", "raise:bbbbb", "could:ybybb", "--hard", "--json")
	if code != 0 || json.Unmarshal([]byte(out), &v) != nil || len(v.Guesses) == 0 || len(v.Candidates) == 0 {
		t.Fatalf("json: %d\n%s", code, out)
	}
	for _, args := range [][]string{{"next", "slate"}, {"next", "slate:bbbqb"}, {"next", "qqqqq:bbbbb"}} {
		if code, _, errOut := run(t, args...); code != 2 || errOut == "" {
			t.Errorf("%v: code %d", args, code)
		}
	}
}
