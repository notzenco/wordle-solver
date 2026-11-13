package wordle

import (
	"math"
	"math/rand/v2"
	"sync"
)

// A Strategy picks each guess deterministically from the position, using
// the answer list as both the possible answers and the allowed guesses.
type Strategy struct {
	Name        string // as given to bench -strategy
	Description string
	next        func(p *position) int // index into Answers
}

// position is what a strategy sees: the turns so far and the answers that
// still fit them.
type position struct {
	history []Turn
	cands   []int
	hard    bool
	max     int // guess limit
}

// Strategies are the guess-selection rules compared by bench -strategy.
var Strategies = []Strategy{
	{"entropy", "maximum information: the most bits of entropy over the colour patterns", byScore(func(b *[243]int, n int) (float64, float64) {
		return -entropy(b, n), 0
	})},
	{"expected", "fewest words expected to remain", fewestExpected},
	{"minimax", "best worst case: the smallest largest group left, then fewest expected", byScore(func(b *[243]int, n int) (float64, float64) {
		return float64(worstLeft(b)), meanLeft(b, n)
	})},
	{"freq", "letter frequency: the fitting word whose letters appear in the most fitting words", letterFrequency},
	{"planner", "look-ahead: the most answers guaranteed within three guesses, then within two", planner},
}

// StrategyByName finds one of Strategies.
func StrategyByName(name string) (Strategy, bool) {
	for _, st := range Strategies {
		if st.Name == name {
			return st, true
		}
	}
	return Strategy{}, false
}

// The pattern table holds scoreCode for every pair of answer-list words,
// so strategies look patterns up instead of computing them.
var (
	tableOnce sync.Once
	table     []uint8
	wordIndex = func() map[string]int {
		m := make(map[string]int, len(Answers))
		for i, w := range Answers {
			m[w] = i
		}
		return m
	}()
)

func code(guess, answer int) int {
	tableOnce.Do(func() {
		n := len(Answers)
		table = make([]uint8, n*n)
		for g := range n {
			for a := range n {
				table[g*n+a] = uint8(scoreCode(Answers[g], Answers[a]))
			}
		}
	})
	return int(table[guess*len(Answers)+answer])
}

// decode turns a scoreCode back into a pattern such as "bygbb".
func decode(c int) string {
	var p [5]byte
	for i := range 5 {
		p[i] = [3]byte{Grey, Yellow, Green}[c%3]
		c /= 3
	}
	return string(p[:])
}

// entropy is the information in bits that a guess's pattern gives, when
// b counts how many of n candidates produce each pattern.
func entropy(b *[243]int, n int) float64 {
	h := 0.0
	for _, k := range b {
		if k > 0 {
			p := float64(k) / float64(n)
			h -= p * math.Log2(p)
		}
	}
	return h
}

// meanLeft is how many candidates are expected to remain, counting a win
// as none left; Solver.expectedLeft computes the same on words.
func meanLeft(b *[243]int, n int) float64 {
	sum := 0
	for c, k := range b {
		if c != solvedCode {
			sum += k * k
		}
	}
	return float64(sum) / float64(n)
}

// worstLeft is the most candidates any pattern can leave.
func worstLeft(b *[243]int) int {
	worst := 0
	for c, k := range b {
		if c != solvedCode {
			worst = max(worst, k)
		}
	}
	return worst
}

// byScore makes a strategy that tries every allowed guess and keeps the one
// with the lowest (primary, secondary) score. Ties go to a word that could
// still be the answer, then to the earlier word in the list.
func byScore(score func(b *[243]int, n int) (float64, float64)) func(*position) int {
	const eps = 1e-9
	return func(p *position) int {
		if len(p.cands) == 1 {
			return p.cands[0]
		}
		isCand := make([]bool, len(Answers))
		for _, c := range p.cands {
			isCand[c] = true
		}
		best, bestCand := -1, false
		var best1, best2 float64
		for g := range Answers {
			if !allowedGuess(g, p) {
				continue
			}
			var b [243]int
			for _, a := range p.cands {
				b[code(g, a)]++
			}
			s1, s2 := score(&b, len(p.cands))
			better := best < 0 || s1 < best1-eps ||
				(s1 <= best1+eps && (s2 < best2-eps || (s2 <= best2+eps && isCand[g] && !bestCand)))
			if better {
				best, bestCand, best1, best2 = g, isCand[g], s1, s2
			}
		}
		return best
	}
}

// allowedGuess rules out repeats, which teach nothing, and in Hard Mode any
// guess that ignores a revealed letter.
func allowedGuess(g int, p *position) bool {
	for _, t := range p.history {
		if t.Guess == Answers[g] {
			return false
		}
	}
	return !p.hard || HardValid(Answers[g], p.history)
}

// letterFrequency is the classic hand heuristic: guess the fitting word
// whose distinct letters occur in the most fitting words.
func letterFrequency(p *position) int {
	var count [26]int
	for _, c := range p.cands {
		var seen [26]bool
		for _, r := range []byte(Answers[c]) {
			if !seen[r-'a'] {
				seen[r-'a'] = true
				count[r-'a']++
			}
		}
	}
	best, bestScore := p.cands[0], -1
	for _, c := range p.cands {
		var seen [26]bool
		score := 0
		for _, r := range []byte(Answers[c]) {
			if !seen[r-'a'] {
				seen[r-'a'] = true
				score += count[r-'a']
			}
		}
		if score > bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

var fewestExpected = byScore(func(b *[243]int, n int) (float64, float64) { return meanLeft(b, n), 0 })

// planner applies the Solver's look-ahead every turn: with three or more
// guesses left it maximises the answers guaranteed within three guesses
// (bestForThree), and with two left those guaranteed within two
// (bestForTwo). Looking ahead from the first guess is too slow, so it opens
// like the expected strategy.
func planner(p *position) int {
	if len(p.history) == 0 {
		return fewestExpected(p)
	}
	if len(p.cands) == 1 {
		return p.cands[0]
	}
	cands := make([]string, len(p.cands))
	for i, c := range p.cands {
		cands[i] = Answers[c]
	}
	// The Solver samples candidates when ranking more than sampleLimit, so
	// a fixed seed keeps each decision the same every time.
	s := &Solver{Max: p.max, Hard: p.hard, History: p.history, words: Answers, rng: rand.New(rand.NewPCG(1, 1))}
	turnsLeft := p.max + 1 - s.Turn()
	switch {
	case turnsLeft <= 1:
		return p.cands[0]
	case turnsLeft >= 3 && len(cands) > 3:
		return wordIndex[s.bestForThree(cands, s.rank(cands, cands))]
	default:
		g, _ := s.bestForTwo(cands)
		return wordIndex[g]
	}
}

// Split describes how a guess divides the whole answer list.
type Split struct {
	Bits     float64 // entropy of the colour pattern
	Expected float64 // answers expected to remain
	Worst    int     // most answers any pattern can leave
}

// OpenerStats measures word as a first guess against every answer.
func OpenerStats(word string) Split {
	g := wordIndex[word]
	var b [243]int
	for a := range Answers {
		b[code(g, a)]++
	}
	return Split{entropy(&b, len(Answers)), meanLeft(&b, len(Answers)), worstLeft(&b)}
}

// InAnswers reports whether w is on the answer list the strategies use.
func InAnswers(w string) bool {
	_, ok := wordIndex[w]
	return ok
}

// Suggest picks the strategy's next guess for a game in progress, using only
// the clues in history, which may include guesses outside the answer list.
// When the clues rule out every listed answer, as for newer answers, it
// suggests the most familiar valid word that still fits.
func (st Strategy) Suggest(history []Turn, hard bool) (string, bool) {
	p := &position{history: history, hard: hard, max: 6}
	for i, a := range Answers {
		if len(Filter([]string{a}, history)) == 1 {
			p.cands = append(p.cands, i)
		}
	}
	if len(p.cands) > 0 {
		return Answers[st.next(p)], true
	}
	best, bestScore := "", -1.0
	for _, w := range Filter(ValidWords(), history) {
		if hard && !HardValid(w, history) {
			continue
		}
		if f := familiarity(w); f > bestScore {
			best, bestScore = w, f
		}
	}
	return best, best != ""
}
