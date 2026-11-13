package player

import (
	"math"
	"math/rand/v2"
	"slices"
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

func TestTargetFillsGapsInTheRecord(t *testing.T) {
	s, _ := Load(t.TempDir(), rand.New(rand.NewPCG(5, 5)))
	for n := range 20 {
		s.Record(n, "", game(n, 3))
	}
	rng := rand.New(rand.NewPCG(6, 6))
	for range 20 {
		if got := s.Target(rng, 6); got == 3 {
			t.Fatal("drew another 3 after twenty 3s in a row")
		}
	}
}

func TestTargetsTrackATypicalSpread(t *testing.T) {
	s, _ := Load(t.TempDir(), rand.New(rand.NewPCG(7, 7)))
	rng := rand.New(rand.NewPCG(8, 8))
	const games = 300
	for n := range games {
		s.Record(n, "", game(n, s.Target(rng, 6)))
	}
	counts := map[int]int{}
	for _, g := range s.History {
		counts[len(g.Turns)]++
	}
	for turn, w := range wordle.FinishWeights {
		got := float64(counts[turn]) / games
		if math.Abs(got-w) > 0.02 {
			t.Errorf("turn %d: %.3f of games, want %.3f ± 0.02", turn, got, w)
		}
	}
}

func TestTargetRespectsLimit(t *testing.T) {
	s, _ := Load(t.TempDir(), rand.New(rand.NewPCG(9, 9)))
	rng := rand.New(rand.NewPCG(10, 10))
	for range 200 {
		if got := s.Target(rng, 4); got < 2 || got > 4 {
			t.Fatalf("target %d outside 2..4", got)
		}
	}
}
