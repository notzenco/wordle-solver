package wordle

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestScoreRepeatedLetters(t *testing.T) {
	cases := []struct{ guess, answer, want string }{
		{"speed", "abide", "bbyby"},
		{"geese", "eerie", "bgybg"},
		{"allee", "eagle", "yybyg"},
		{"crane", "crane", Solved},
	}
	for _, c := range cases {
		if got := Score(c.guess, c.answer); got != c.want {
			t.Errorf("Score(%s, %s) = %s, want %s", c.guess, c.answer, got, c.want)
		}
	}
}

func TestParsePattern(t *testing.T) {
	for _, in := range []string{"gybbg", "21002", "🟩🟨⬛⬛🟩", "g y b b g"} {
		if got, err := ParsePattern(in); err != nil || got != "gybbg" {
			t.Errorf("ParsePattern(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := ParsePattern("gyb"); err == nil {
		t.Error("short pattern accepted")
	}
}

func TestEveryGuessIsACommonWord(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 1))
	for _, i := range rng.Perm(len(Answers))[:200] {
		for _, turn := range Play(Answers[i], rng, 0, 0) {
			if !slices.Contains(Answers, turn.Guess) && !slices.Contains(Openers, turn.Guess) {
				t.Fatalf("guessed uncommon word %q", turn.Guess)
			}
		}
	}
}

func TestNeverFails(t *testing.T) {
	rng := rand.New(rand.NewPCG(2, 2))
	failed := 0
	for _, answer := range Answers {
		h := Play(answer, rng, 0, 0)
		if h[len(h)-1].Pattern != Solved {
			failed++
			t.Logf("failed on %s: %v", answer, h)
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d games lost", failed, len(Answers))
	}
}

func TestRarelyWinsBeforeTarget(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 3))
	early := 0
	for _, i := range rng.Perm(len(Answers))[:300] {
		if len(Play(Answers[i], rng, 4, 0)) < 4 {
			early++
		}
	}
	if early > 30 {
		t.Errorf("won before turn 4 in %d of 300 games", early)
	}
}

func TestOpenersVary(t *testing.T) {
	seen := map[string]bool{}
	for i := range uint64(40) {
		g, _ := NewSolver(rand.New(rand.NewPCG(i, i)), 0, 0).NextGuess()
		seen[g] = true
	}
	if len(seen) < 10 {
		t.Errorf("only %d distinct openers", len(seen))
	}
}

func TestMaxCapsFinishTurn(t *testing.T) {
	// Even an optimal solver needs a fifth guess for about 1.6% of answers,
	// so a limit of 4 can only be met almost always; 5 always (6 is
	// covered by TestNeverFails).
	allowed := map[int]int{4: len(Answers) * 3 / 100, 5: 0}
	for limit, allow := range allowed {
		rng := rand.New(rand.NewPCG(4, 4))
		late := 0
		for _, answer := range Answers {
			h := Play(answer, rng, 0, limit)
			if len(h) > limit || h[len(h)-1].Pattern != Solved {
				late++
				if allow == 0 {
					t.Logf("max %d: %s took %v", limit, answer, h)
				}
			}
		}
		t.Logf("max %d: %d of %d over the limit", limit, late, len(Answers))
		if late > allow {
			t.Errorf("max %d: %d games went over the limit, allowed %d", limit, late, allow)
		}
	}
}

// slips counts guesses that reuse a ruled-out letter or repeat a yellow in a
// spot it was already yellow in, while keeping every known green.
func slips(h []Turn) int {
	n := 0
	for k := 1; k < len(h); k++ {
		greens, absent, present := map[int]byte{}, map[byte]bool{}, map[byte]bool{}
		yellowAt := map[[2]int]bool{}
		for _, t := range h[:k] {
			for i := range 5 {
				c := t.Guess[i]
				switch t.Pattern[i] {
				case Green:
					greens[i] = c
					present[c] = true
				case Yellow:
					yellowAt[[2]int{i, int(c)}] = true
					present[c] = true
				default:
					absent[c] = true
				}
			}
		}
		g := h[k].Guess
		keepsGreens, slipped := true, false
		for i, c := range greens {
			keepsGreens = keepsGreens && g[i] == c
		}
		for i := range 5 {
			slipped = slipped || (absent[g[i]] && !present[g[i]]) || yellowAt[[2]int{i, int(g[i])}]
		}
		if keepsGreens && slipped && g != h[len(h)-1].Guess {
			n++
		}
	}
	return n
}

func TestSometimesSlipsLikeAPerson(t *testing.T) {
	count := func(sloppiness float64) int {
		rng := rand.New(rand.NewPCG(6, 6))
		games := 0
		for _, i := range rng.Perm(len(Answers))[:300] {
			s := NewSolver(rng, 0, 0)
			s.Sloppiness = sloppiness
			if slips(playWith(s, Answers[i])) > 0 {
				games++
			}
		}
		return games
	}
	base, sloppy := count(0), count(0.25)
	t.Logf("games with a slip: %d without sloppiness, %d with", base, sloppy)
	if sloppy < base+30 {
		t.Errorf("sloppiness added too few slips: %d -> %d of 300 games", base, sloppy)
	}
}
