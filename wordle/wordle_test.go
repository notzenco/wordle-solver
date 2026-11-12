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

func TestScoreCodeMatchesScore(t *testing.T) {
	marks := map[byte]int{Grey: 0, Yellow: 1, Green: 2}
	rng := rand.New(rand.NewPCG(9, 9))
	for range 20000 {
		g, a := Answers[rng.IntN(len(Answers))], Answers[rng.IntN(len(Answers))]
		want, p := 0, Score(g, a)
		for i, pow := 0, 1; i < 5; i, pow = i+1, pow*3 {
			want += marks[p[i]] * pow
		}
		if got := scoreCode(g, a); got != want {
			t.Fatalf("scoreCode(%s, %s) = %d, want %d (%s)", g, a, got, want, p)
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
	t.Parallel()
	failed := 0
	for i, h := range playAll(Answers, 2, func(rng *rand.Rand) *Solver { return NewSolver(rng, 0, 0) }) {
		if h[len(h)-1].Pattern != Solved {
			failed++
			t.Logf("failed on %s: %v", Answers[i], h)
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
	// so a limit of 4 can only be met almost always. A limit of 5 is missed
	// about once in 20,000 games: guess 2 can lead into an _O_ER cluster
	// that no third guess fully cracks, and looking further ahead from guess
	// 2 would be too slow. Allow at most 1 miss in these 6,945 games (6 is
	// covered by TestNeverFails).
	t.Parallel()
	allowed := map[int]int{4: len(Answers) * 3 / 100, 5: 0}
	for limit, allow := range allowed {
		late := 0
		var games [][]Turn
		for seed := range uint64(3) {
			games = append(games, playAll(Answers, 40+seed, func(rng *rand.Rand) *Solver { return NewSolver(rng, 0, limit) })...)
		}
		allow *= 3
		if limit == 5 {
			allow = 1
		}
		for i, h := range games {
			if len(h) > limit || h[len(h)-1].Pattern != Solved {
				late++
				if limit == 5 {
					t.Logf("max %d: %s took %v", limit, Answers[i%len(Answers)], h)
				}
			}
		}
		t.Logf("max %d: %d of %d over the limit", limit, late, len(games))
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
			if slips(s.Solve(Answers[i])) > 0 {
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

func TestPrefersEverydayWords(t *testing.T) {
	s := NewSolver(rand.New(rand.NewPCG(7, 7)), 0, 0)
	house := 0
	for range 2000 {
		if s.choose([]string{"house", "nadir"}) == "house" {
			house++
		}
	}
	if house < 1600 {
		t.Errorf("chose HOUSE over NADIR only %d of 2000 times", house)
	}
}

func TestHardModeGuessesFollowTheRules(t *testing.T) {
	t.Parallel()
	games := playAll(Answers, 8, func(rng *rand.Rand) *Solver {
		s := NewSolver(rng, 0, 0)
		s.Hard = true
		return s
	})
	lost := 0
	for i, h := range games {
		for k := 1; k < len(h); k++ {
			if !HardValid(h[k].Guess, h[:k]) {
				t.Fatalf("%s: guess %q breaks hard mode after %v", Answers[i], h[k].Guess, h[:k])
			}
		}
		if h[len(h)-1].Pattern != Solved {
			lost++
		}
	}
	// Hard mode has traps no strategy escapes every time (_IGHT, _ATCH).
	t.Logf("hard mode lost %d of %d", lost, len(games))
	if lost*100 > len(games) {
		t.Errorf("hard mode lost %d of %d games", lost, len(games))
	}
}

func TestHardValid(t *testing.T) {
	h := []Turn{{"crane", "gybbb"}} // C green, R yellow
	cases := map[string]bool{"cider": true, "curry": true, "corny": true, "cloud": false, "rocky": false}
	for w, want := range cases {
		if got := HardValid(w, h); got != want {
			t.Errorf("HardValid(%s) = %v, want %v", w, got, want)
		}
	}
}

func TestOpenerOverride(t *testing.T) {
	s := NewSolver(rand.New(rand.NewPCG(11, 11)), 0, 0)
	s.Opener = "adieu"
	if g, _ := s.NextGuess(); g != "adieu" {
		t.Errorf("first guess %q, want the chosen opener", g)
	}
	s = NewSolver(rand.New(rand.NewPCG(11, 11)), 0, 4)
	s.Opener = "adieu"
	if g, _ := s.NextGuess(); g == "adieu" || !slices.Contains(StrongOpeners, g) {
		t.Errorf("under -max 4 first guess %q, want a strong opener instead of ADIEU", g)
	}
}

func TestAddWords(t *testing.T) {
	s := NewSolver(rand.New(rand.NewPCG(12, 12)), 0, 0)
	before := len(s.Candidates())
	s.AddWords("moxie", "house", "moxie")
	cands := s.Candidates()
	if len(cands) != before+1 || !slices.Contains(cands, "moxie") {
		t.Errorf("want MOXIE added once to %d candidates, got %d", before, len(cands))
	}
	if len(Answers) != before {
		t.Error("AddWords changed the shared Answers list")
	}
}

func TestExcludeIsSoft(t *testing.T) {
	s := NewSolver(rand.New(rand.NewPCG(13, 13)), 0, 0)
	s.Exclude("cigar", "rebut")
	if c := s.Candidates(); slices.Contains(c, "cigar") || len(c) != len(Answers)-2 {
		t.Errorf("excluded words still candidates (%d left)", len(c))
	}
	// A repeated answer must still be found once nothing else fits.
	s = NewSolver(rand.New(rand.NewPCG(14, 14)), 0, 0)
	s.Exclude("cigar")
	if h := s.Solve("cigar"); h[len(h)-1].Pattern != Solved {
		t.Errorf("never found the repeated answer: %v", h)
	}
}
