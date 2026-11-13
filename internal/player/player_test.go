package player

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

func game(n, turns int) []wordle.Turn {
	h := make([]wordle.Turn, turns)
	for i := range h {
		h[i] = wordle.Turn{Guess: "crane", Pattern: "bbbbb"}
	}
	h[turns-1].Pattern = wordle.Solved
	return h
}

func TestLoadCreatesAndKeepsProfile(t *testing.T) {
	dir := t.TempDir()
	rng := rand.New(rand.NewPCG(1, 1))
	s, err := Load(dir, rng)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Profile.Openers) != 2 || s.Profile.Openers[0] == s.Profile.Openers[1] {
		t.Fatalf("want two distinct favourite openers, got %v", s.Profile.Openers)
	}
	for _, o := range s.Profile.Openers {
		// Weak openers are swapped out under -hard and -max, which would
		// make the player look inconsistent.
		if !slices.Contains(wordle.StrongOpeners, o) {
			t.Errorf("favourite opener %q is not one of wordle.StrongOpeners", o)
		}
	}
	s.Record(1973, "2026-11-13", game(1973, 4))
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	again, err := Load(dir, rand.New(rand.NewPCG(2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(again.Profile.Openers, s.Profile.Openers) {
		t.Errorf("profile changed between runs: %v -> %v", s.Profile.Openers, again.Profile.Openers)
	}
	g, ok := again.Game(1973)
	if !ok || g.Date != "2026-11-13" || len(g.Turns) != 4 {
		t.Errorf("recorded game not kept: %+v %v", g, ok)
	}
}

func TestOpenerFavoursTheFirstFavourite(t *testing.T) {
	s, _ := Load(t.TempDir(), rand.New(rand.NewPCG(3, 3)))
	rng := rand.New(rand.NewPCG(4, 4))
	first := 0
	for range 1000 {
		o := s.Opener(rng)
		if !slices.Contains(s.Profile.Openers, o) {
			t.Fatalf("opener %q is not a favourite", o)
		}
		if o == s.Profile.Openers[0] {
			first++
		}
	}
	if first < 700 || first > 900 {
		t.Errorf("first favourite used %d of 1000 times, want about 800", first)
	}
}

func TestLoadTreatsNullSavedDataAsEmpty(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{historyFile, answersFile} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("null\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Load(dir, rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatal(err)
	}
	s.Record(1, "2021-06-20", []wordle.Turn{{Guess: "crane", Pattern: wordle.Solved}})
	s.AddAnswer(1, "crane")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsMalformedSavedGames(t *testing.T) {
	dir := t.TempDir()
	bad := `{"1":{"date":"2021-06-20","turns":[{"Guess":"12!34","Pattern":"ggggg"}]}}`
	if err := os.WriteFile(filepath.Join(dir, historyFile), []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, rand.New(rand.NewPCG(1, 1))); err == nil || !strings.Contains(err.Error(), historyFile) {
		t.Fatalf("malformed game accepted: %v", err)
	}
}

func TestLoadReplacesMalformedOpeners(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, profileFile), []byte(`{"openers":["12!34","crane"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir, rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range s.Profile.Openers {
		if !wordle.IsWord(o) {
			t.Fatalf("kept opener %q", o)
		}
	}
}

func TestOverlappingSavesKeepEachOthersGames(t *testing.T) {
	dir := t.TempDir()
	a, _ := Load(dir, rand.New(rand.NewPCG(1, 1)))
	b, _ := Load(dir, rand.New(rand.NewPCG(2, 2)))
	a.Record(1, "2021-06-20", []wordle.Turn{{Guess: "crane", Pattern: wordle.Solved}})
	a.AddAnswer(1, "crane")
	b.Record(2, "2021-06-21", []wordle.Turn{{Guess: "slate", Pattern: wordle.Solved}})
	b.AddAnswer(2, "slate")
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir, rand.New(rand.NewPCG(3, 3)))
	if err != nil || len(s.History) != 2 || len(s.Answers) != 2 {
		t.Fatalf("history %v, answers %v, err %v", s.History, s.Answers, err)
	}
	mine := map[int]Game{1: {Date: "2021-06-20", Turns: []wordle.Turn{{Guess: "crane", Pattern: wordle.Solved}}}}
	other := map[int]Game{2: {Date: "2021-06-21", Turns: []wordle.Turn{{Guess: "slate", Pattern: wordle.Solved}}}}
	if err := SaveMine(dir, mine); err != nil {
		t.Fatal(err)
	}
	if err := SaveMine(dir, other); err != nil {
		t.Fatal(err)
	}
	if got, _ := LoadMine(dir); len(got) != 2 {
		t.Fatalf("own games: %v", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		info, _ := e.Info()
		if strings.HasSuffix(e.Name(), ".tmp") || strings.HasPrefix(e.Name(), ".") {
			t.Errorf("left %s behind", e.Name())
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s is readable by others: %v", e.Name(), info.Mode().Perm())
		}
	}
}
