package main

import (
	"strings"
	"testing"
)

func TestAutoHeaderDisclosesAnswerLookup(t *testing.T) {
	h := autoHeader(1973, "2026-11-13")
	for _, want := range []string{"#1,973", "2026-11-13", "answer was looked up", "only to score"} {
		if !strings.Contains(h, want) {
			t.Errorf("header lacks %q:\n%s", want, h)
		}
	}
}
