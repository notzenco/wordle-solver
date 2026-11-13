package player

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

func lost() []wordle.Turn {
	h := make([]wordle.Turn, 6)
	for i := range h {
		h[i] = wordle.Turn{Guess: "crane", Pattern: "bbbbb"}
	}
	return h
}

func TestStats(t *testing.T) {
	s, _ := Load(t.TempDir(), rand.New(rand.NewPCG(1, 1)))
	// 100-102 solved, 103 lost, 104-107 solved, 108 not yet played.
	for n := 100; n <= 107; n++ {
		s.Record(n, "", game(n, 3+n%3))
	}
	s.Record(103, "", lost())

	st := s.Stats(108)
	if st.Played != 8 || st.Won != 7 {
		t.Errorf("played %d won %d, want 8 and 7", st.Played, st.Won)
	}
	if st.CurrentStreak != 4 || st.MaxStreak != 4 {
		t.Errorf("streaks %d/%d, want current 4 (today unplayed) and max 4", st.CurrentStreak, st.MaxStreak)
	}
	if st.Guesses[3]+st.Guesses[4]+st.Guesses[5] != 7 {
		t.Errorf("guess spread %v does not add up to 7 wins", st.Guesses)
	}
	if got := s.Stats(110).CurrentStreak; got != 0 {
		t.Errorf("current streak %d after missing a day, want 0", got)
	}
}

func TestAnswerCache(t *testing.T) {
	dir := t.TempDir()
	s, _ := Load(dir, rand.New(rand.NewPCG(2, 2)))
	s.AddAnswer(0, "cigar")
	s.AddAnswer(2, "sissy")
	if got := s.Missing(3); !slices.Equal(got, []int{1, 3}) {
		t.Errorf("Missing(3) = %v, want [1 3]", got)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	again, _ := Load(dir, rand.New(rand.NewPCG(3, 3)))
	if got := again.KnownAnswers(); !slices.Equal(got, []string{"cigar", "sissy"}) {
		t.Errorf("KnownAnswers after reload = %v", got)
	}
}

func TestAnswersBefore(t *testing.T) {
	s, _ := Load(t.TempDir(), rand.New(rand.NewPCG(4, 4)))
	s.AddAnswer(0, "cigar")
	s.AddAnswer(1, "rebut")
	s.AddAnswer(2, "sissy")
	if got := s.AnswersBefore(2); !slices.Equal(got, []string{"cigar", "rebut"}) {
		t.Errorf("AnswersBefore(2) = %v", got)
	}
}

func TestSomeProfilesSkipPastAnswers(t *testing.T) {
	skip := 0
	for i := range uint64(40) {
		s, _ := Load(t.TempDir(), rand.New(rand.NewPCG(i, i)))
		if s.Profile.SkipPastAnswers {
			skip++
		}
	}
	if skip < 10 || skip > 30 {
		t.Errorf("%d of 40 new players skip past answers, want about half", skip)
	}
}
