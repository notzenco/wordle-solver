package wordle

import (
	"math"
	"slices"
	"testing"
)

func TestBucketScores(t *testing.T) {
	// Four candidates split into: the guess itself, one other word alone,
	// and a pair.
	var b [243]int
	b[solvedCode] = 1
	b[0] = 1
	b[5] = 2
	if got := entropy(&b, 4); math.Abs(got-1.5) > 1e-12 {
		t.Errorf("entropy = %v, want 1.5 bits", got)
	}
	if got := meanLeft(&b, 4); got != 1.25 {
		t.Errorf("meanLeft = %v, want (1+4)/4", got)
	}
	if got := worstLeft(&b); got != 2 {
		t.Errorf("worstLeft = %v, want 2", got)
	}
}

func TestDecodeMatchesScore(t *testing.T) {
	for _, pair := range [][2]string{{"crane", "nacre"}, {"speed", "abide"}, {"geese", "eerie"}, {"cigar", "cigar"}} {
		if got, want := decode(scoreCode(pair[0], pair[1])), Score(pair[0], pair[1]); got != want {
			t.Errorf("decode(scoreCode(%s, %s)) = %s, want %s", pair[0], pair[1], got, want)
		}
	}
}

// In the _OUND trap, guessing one fitting word per turn can lose, so the
// search strategies spend a guess on a word that splits the group, while
// the letter-frequency heuristic only ever guesses fitting words.
func TestTrapSplitting(t *testing.T) {
	trap := []string{"bound", "found", "hound", "mound", "pound", "round", "sound", "wound"}
	var cands []int
	for _, w := range trap {
		cands = append(cands, wordIndex[w])
	}
	history := []Turn{{"cloud", Score("cloud", "found")}}
	for _, st := range Strategies {
		g := Answers[st.next(&position{history: history, cands: cands, max: 6})]
		fits := slices.Contains(trap, g)
		if st.Name == "freq" && !fits {
			t.Errorf("freq guessed %s, which does not fit", g)
		}
		if st.Name != "freq" && fits {
			t.Errorf("%s guessed fitting word %s instead of splitting the group", st.Name, g)
		}
		if st.Name == "minimax" {
			if got, want := worstAfter(wordIndex[g], cands), bestWorst(cands); got != want {
				t.Errorf("minimax guessed %s, which can leave %d words; the best guess leaves %d", g, got, want)
			}
		}
	}
}

func worstAfter(g int, cands []int) int {
	var b [243]int
	for _, a := range cands {
		b[code(g, a)]++
	}
	return worstLeft(&b)
}

func bestWorst(cands []int) int {
	best := len(cands)
	for g := range Answers {
		best = min(best, worstAfter(g, cands))
	}
	return best
}

func TestStrategiesSolveInHardMode(t *testing.T) {
	sample := Answers[:60]
	for _, st := range Strategies {
		r := Benchmark(st, true, sample)
		for _, g := range r.Games {
			for i, turn := range g.Turns {
				if !HardValid(turn.Guess, g.Turns[:i]) {
					t.Fatalf("%s: %s breaks Hard Mode after %v", st.Name, turn.Guess, g.Turns[:i])
				}
			}
		}
	}
}

// Benchmark caches each position's decision; that must not change a game.
func TestBenchmarkMatchesPlay(t *testing.T) {
	sample := Answers[100:140]
	for _, st := range Strategies {
		for _, hard := range []bool{false, true} {
			r := Benchmark(st, hard, sample)
			for i, a := range sample {
				if got := st.Play(a, hard); !slices.Equal(got, r.Games[i].Turns) {
					t.Fatalf("%s hard=%v %s: Play %v, Benchmark %v", st.Name, hard, a, got, r.Games[i].Turns)
				}
			}
		}
	}
}

func TestBenchmarkTotals(t *testing.T) {
	sample := Answers[:80]
	for _, st := range Strategies {
		r := Benchmark(st, false, sample)
		total := r.Failed
		turns := 0
		for n, c := range r.Wins {
			total += c
			turns += n * c
		}
		if total != len(sample) || len(r.Games) != len(sample) {
			t.Errorf("%s: %d wins+losses, %d games, want %d", st.Name, total, len(r.Games), len(sample))
		}
		if won := len(sample) - r.Failed; won > 0 && math.Abs(r.Mean-float64(turns)/float64(won)) > 1e-12 {
			t.Errorf("%s: mean %v, want %v", st.Name, r.Mean, float64(turns)/float64(won))
		}
		if r.Opener != r.Games[0].Turns[0].Guess {
			t.Errorf("%s: opener %s, but the first game opened with %s", st.Name, r.Opener, r.Games[0].Turns[0].Guess)
		}
	}
}

func TestStrategyByName(t *testing.T) {
	for _, st := range Strategies {
		if got, ok := StrategyByName(st.Name); !ok || got.Name != st.Name {
			t.Errorf("StrategyByName(%q) = %v, %v", st.Name, got.Name, ok)
		}
	}
	if _, ok := StrategyByName("nope"); ok {
		t.Error("found a strategy called nope")
	}
}

func TestOpenerStats(t *testing.T) {
	all := make([]int, len(Answers))
	for i := range all {
		all[i] = i
	}
	st, _ := StrategyByName("minimax")
	opener := Answers[st.next(&position{cands: all, max: 6})]
	got := OpenerStats(opener)
	if got.Worst != bestWorst(all) {
		t.Errorf("OpenerStats(%s).Worst = %d, want the best possible %d", opener, got.Worst, bestWorst(all))
	}
	if got.Bits <= 0 || got.Bits > math.Log2(243) || got.Expected <= 0 || got.Expected > float64(got.Worst) {
		t.Errorf("OpenerStats(%s) = %+v is out of range", opener, got)
	}
}
