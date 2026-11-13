package wordle

import (
	"strings"
	"time"
)

// Game is one benchmark game.
type Game struct {
	Answer string
	Turns  []Turn
}

// BenchResult summarises a strategy's games over a set of answers.
type BenchResult struct {
	Strategy string
	Hard     bool
	Opener   string
	Wins     [7]int // games won in 1..6 guesses, by index
	Failed   int    // games not solved within 6 guesses
	Mean     float64
	// Runtime is the time to play every game. Decisions are cached by
	// position, since a deterministic strategy always plays the same guess
	// from the same position; Positions counts the decisions computed.
	Runtime   time.Duration
	Positions int
	Games     []Game
}

// Play plays one game against answer, as Benchmark would.
func (st Strategy) Play(answer string, hard bool) []Turn {
	return st.play(answer, hard, nil)
}

// Benchmark plays every answer with st, within six guesses, and reports the
// results. Answers outside the answer list are not supported.
func Benchmark(st Strategy, hard bool, answers []string) BenchResult {
	r := BenchResult{Strategy: st.Name, Hard: hard}
	cache := map[string]int{}
	code(0, 0) // build the pattern table outside the timed run
	start := time.Now()
	for _, a := range answers {
		turns := st.play(a, hard, cache)
		r.Games = append(r.Games, Game{a, turns})
	}
	r.Runtime = time.Since(start)
	r.Positions = len(cache)
	turns := 0
	for _, g := range r.Games {
		if n := len(g.Turns); g.Turns[n-1].Pattern == Solved {
			r.Wins[n]++
			turns += n
		} else {
			r.Failed++
		}
	}
	if won := len(answers) - r.Failed; won > 0 {
		r.Mean = float64(turns) / float64(won)
	}
	if len(r.Games) > 0 {
		r.Opener = r.Games[0].Turns[0].Guess
	}
	return r
}

// play runs one game. With a cache it reuses decisions for positions seen
// in earlier games.
func (st Strategy) play(answer string, hard bool, cache map[string]int) []Turn {
	ans := wordIndex[answer]
	p := &position{hard: hard, max: 6, cands: make([]int, len(Answers))}
	for i := range p.cands {
		p.cands[i] = i
	}
	var key strings.Builder
	key.WriteString(map[bool]string{false: "n:", true: "h:"}[hard])
	for len(p.history) < 6 {
		g, ok := cache[key.String()]
		if !ok {
			g = st.next(p)
			if cache != nil {
				cache[key.String()] = g
			}
		}
		c := code(g, ans)
		p.history = append(p.history, Turn{Answers[g], decode(c)})
		if c == solvedCode {
			break
		}
		var left []int
		for _, a := range p.cands {
			if code(g, a) == c {
				left = append(left, a)
			}
		}
		p.cands = left
		key.WriteString(Answers[g] + decode(c))
	}
	return p.history
}
