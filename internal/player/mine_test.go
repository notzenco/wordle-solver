package player

import (
	"testing"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

func TestOwnGamesAreKeptApartFromThePlayerModel(t *testing.T) {
	dir := t.TempDir()
	games, err := LoadMine(dir)
	if err != nil || len(games) != 0 {
		t.Fatal(games, err)
	}
	games[1973] = Game{Date: "2026-11-13", Turns: []wordle.Turn{{Guess: "crane", Pattern: wordle.Solved}}}
	if err := SaveMine(dir, games); err != nil {
		t.Fatal(err)
	}
	again, err := LoadMine(dir)
	if err != nil || !again[1973].Solved() {
		t.Fatal(again, err)
	}
	removed, err := Clean(dir)
	if err != nil || len(removed) != 1 {
		t.Fatalf("clean removed %v, %v", removed, err)
	}
}
