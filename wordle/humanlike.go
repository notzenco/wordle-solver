package wordle

import (
	"cmp"
	"errors"
	"math/rand/v2"
	"slices"
)

// Solver plays like a decent human rather than an optimiser. Bots give
// themselves away in a few ways, and this avoids each one:
//   - Obscure "optimal" words (SOARE, TARSE): every guess is a common answer word.
//   - Probes that ignore known letters: guesses fit every earlier hint
//     (hard-mode style), apart from the occasional near-miss below.
//   - The same opener every day: openers rotate through popular human picks.
//   - Flawless play: now and then, with guesses to spare, it slips like a
//     person, reusing a ruled-out letter or a yellow in the same spot.
//   - Suspiciously consistent 2s and 3s: a finishing turn is drawn from a
//     human-like distribution. When ahead of it the solver spends turns on
//     elimination words or near-misses instead of winning early.
type Solver struct {
	Target int
	Max    int // must solve within this many guesses
	// Sloppiness is the chance, on a guess with turns to spare, of a human
	// slip: reusing a ruled-out letter or repeating a yellow in the same spot.
	Sloppiness float64
	History    []Turn
	words      []string // possible answers
	rng        *rand.Rand
}

var Openers = []string{
	"adieu", "audio", "crane", "slate", "stare", "raise", "arise", "house",
	"train", "roast", "irate", "later", "trace", "crate", "stone", "heart",
	"great", "least", "about", "spare", "react", "alert", "steam", "plane",
}

// Popular openers that split the answers well enough to finish within four
// guesses almost every time; weaker favourites like ADIEU miss about 1 in 10.
var strongOpeners = []string{
	"stare", "slate", "roast", "heart", "trace", "steam", "spare", "react",
	"later", "crane", "train",
}

// DefaultSloppiness is the Sloppiness new solvers start with.
var DefaultSloppiness = 0.2

// Roughly the results of a regular player, for finishing on turns 2 to 6.
var finishWeights = []float64{0.04, 0.27, 0.40, 0.22, 0.07}

// Above this many candidates, rank guesses against a sample to stay fast.
const sampleLimit = 400

var ErrNoCandidates = errors.New("no common word fits that feedback; check the patterns")

// NewSolver aims to finish on target, or on a randomly drawn turn when
// target is 0, and never later than limit guesses (6 when limit is 0).
func NewSolver(rng *rand.Rand, target, limit int) *Solver {
	if limit == 0 {
		limit = 6
	}
	if target == 0 {
		target = drawTarget(rng, limit)
	}
	return &Solver{Target: min(target, limit), Max: limit, Sloppiness: DefaultSloppiness, words: Answers, rng: rng}
}

// drawTarget samples the human finishing spread, cut off at limit.
func drawTarget(rng *rand.Rand, limit int) int {
	weights := finishWeights[:max(min(limit-1, len(finishWeights)), 1)]
	total := 0.0
	for _, w := range weights {
		total += w
	}
	x := rng.Float64() * total
	for i, w := range weights {
		if x < w {
			return i + 2
		}
		x -= w
	}
	return len(weights) + 1
}

func (s *Solver) Turn() int { return len(s.History) + 1 }

func (s *Solver) Candidates() []string { return Filter(s.words, s.History) }

func (s *Solver) Record(guess, pattern string) {
	s.History = append(s.History, Turn{guess, pattern})
}

func (s *Solver) NextGuess() (string, error) {
	if len(s.History) == 0 {
		openers := Openers
		if s.Max <= 4 {
			openers = strongOpeners
		}
		return openers[s.rng.IntN(len(openers))], nil
	}
	cands := s.Candidates()
	if len(cands) == 0 {
		return "", ErrNoCandidates
	}
	ahead := s.Turn() < s.Target
	if len(cands) == 1 {
		if ahead {
			if w, ok := s.nearMiss(cands[0]); ok {
				return w, nil
			}
		}
		return cands[0], nil
	}
	return s.pick(cands, ahead), nil
}

func (s *Solver) pick(cands []string, ahead bool) string {
	turnsLeft := s.Max + 1 - s.Turn()
	switch turnsLeft {
	case 1:
		return cands[s.rng.IntN(len(cands))]
	case 2:
		g, _ := s.bestForTwo(cands)
		return g
	}
	// With four or more of six guesses left a slip costs little and can't
	// lose. A tighter limit has no room for one (a slip on guess 2 under a
	// limit of 5 can miss it), so the solver then plays carefully.
	if s.Max == 6 && turnsLeft >= 4 && s.rng.Float64() < s.Sloppiness {
		if w, ok := s.slip(cands); ok {
			return w
		}
	}
	ranked := s.rank(cands, cands)
	if turnsLeft == 3 && len(cands) > 3 {
		return s.bestForThree(cands, ranked)
	}

	if ahead && turnsLeft >= 3 && len(cands) <= 12 {
		// Ahead of pace with few words left: spend a turn on an elimination
		// word, as people do, which also can't win too early.
		if pool := s.eliminators(cands, 5); len(pool) > 0 {
			return pool[s.rng.IntN(len(pool))]
		}
	}
	if len(cands) > 2 && len(cands) >= turnsLeft {
		// A trap like _OUND: guessing one fitting word per turn could lose,
		// so take an elimination word unless a fitting word splits as well.
		best := ranked[0]
		if pool := s.eliminators(cands, 1); len(pool) > 0 &&
			s.expectedLeft(pool[0], cands) < s.expectedLeft(best, cands) {
			return pool[0]
		}
		return best
	}

	var pool []string
	switch {
	case !ahead || turnsLeft <= 2:
		pool = ranked[:1]
	case len(cands) > turnsLeft*6:
		// Many words still fit: stay sharp so pacing never risks a loss.
		pool = ranked[:min(3, len(ranked))]
	default:
		// Ahead of pace: a reasonable but not best guess, like a person
		// picking whichever fitting word comes to mind.
		pool = ranked[:max(2, len(ranked)/2)]
	}
	return pool[s.rng.IntN(len(pool))]
}

// slip returns a decent word that keeps every green and every known letter
// but makes exactly one human mistake: a ruled-out letter, or a yellow put
// back where it was already yellow.
func (s *Solver) slip(cands []string) (string, bool) {
	greens := map[int]byte{}
	var present, grey [26]bool
	var yellowAt [5][26]bool
	guessed := map[string]bool{}
	for _, t := range s.History {
		guessed[t.Guess] = true
		for i := range 5 {
			c := t.Guess[i] - 'a'
			switch t.Pattern[i] {
			case Green:
				greens[i] = t.Guess[i]
				present[c] = true
			case Yellow:
				yellowAt[i][c] = true
				present[c] = true
			default:
				grey[c] = true
			}
		}
	}
	var options []string
outer:
	for _, w := range s.words {
		if guessed[w] {
			continue
		}
		for i, c := range greens {
			if w[i] != c {
				continue outer
			}
		}
		var has [26]bool
		mistakes := 0
		for i := range 5 {
			c := w[i] - 'a'
			has[c] = true
			if (grey[c] && !present[c]) || yellowAt[i][c] {
				mistakes++
			}
		}
		for c := range 26 {
			if present[c] && !has[c] {
				continue outer
			}
		}
		if mistakes == 1 {
			options = append(options, w)
		}
	}
	if len(options) == 0 {
		return "", false
	}
	// Among the slips, a reasonably useful word, not a wild one.
	ranked := s.rank(options, cands)
	pool := ranked[:min(8, len(ranked))]
	return pool[s.rng.IntN(len(pool))], true
}

// bestForTwo maximises the chance of solving within this guess and the
// next: it wins now if it is the answer, or next turn if the clue it gets
// leaves a single word. Fitting words win ties, as a person would play them.
// It returns the guess and how many of cands it is sure to solve.
func (s *Solver) bestForTwo(cands []string) (string, int) {
	best, bestWins := cands[0], -1
	consider := func(g string) {
		if wins := singletons(g, cands); wins > bestWins {
			best, bestWins = g, wins
		}
	}
	for _, g := range cands {
		consider(g)
	}
	if bestWins < len(cands) {
		fits := map[string]bool{}
		for _, c := range cands {
			fits[c] = true
		}
		for _, g := range s.words {
			if !fits[g] && bestWins < len(cands) {
				consider(g)
			}
		}
	}
	return best, bestWins
}

// singletons counts the answers in cands that g pins down to one word,
// including g itself when it fits.
func singletons(g string, cands []string) int {
	buckets := map[string]int{}
	for _, a := range cands {
		buckets[Score(g, a)]++
	}
	n := 0
	for _, size := range buckets {
		if size == 1 {
			n++
		}
	}
	return n
}

// bestForThree looks one guess ahead: among the most promising fitting and
// elimination words, it picks the one that guarantees the most answers
// within three guesses, preferring fitting words on a tie.
func (s *Solver) bestForThree(cands, ranked []string) string {
	shortlist := append(slices.Clone(ranked[:min(8, len(ranked))]), s.eliminators(cands, 8)...)
	best, bestWins := shortlist[0], -1
	for _, g := range shortlist {
		buckets := map[string][]string{}
		for _, a := range cands {
			p := Score(g, a)
			buckets[p] = append(buckets[p], a)
		}
		wins := 0
		for p, bucket := range buckets {
			if p == Solved {
				wins++
			} else {
				_, w := s.bestForTwo(bucket)
				wins += w
			}
		}
		if wins > bestWins {
			best, bestWins = g, wins
		}
	}
	return best
}

// eliminators returns up to n unguessed common words that don't fit the
// clues but split cands best.
func (s *Solver) eliminators(cands []string, n int) []string {
	fits := map[string]bool{}
	for _, c := range cands {
		fits[c] = true
	}
	for _, t := range s.History {
		fits[t.Guess] = true
	}
	var others []string
	for _, w := range s.words {
		if !fits[w] {
			others = append(others, w)
		}
	}
	ranked := s.rank(others, cands)
	return ranked[:min(n, len(ranked))]
}

// rank orders guesses by how few of cands they would leave on average.
func (s *Solver) rank(guesses, cands []string) []string {
	scores := make(map[string]float64, len(guesses))
	for _, g := range guesses {
		scores[g] = s.expectedLeft(g, cands)
	}
	ranked := slices.Clone(guesses)
	slices.SortStableFunc(ranked, func(a, b string) int {
		return cmp.Compare(scores[a], scores[b])
	})
	return ranked
}

// expectedLeft is how many candidates would remain on average after
// guessing g, counting an immediate win as none left.
func (s *Solver) expectedLeft(g string, cands []string) float64 {
	sample := cands
	if len(cands) > sampleLimit {
		sample = make([]string, sampleLimit)
		for i, j := range s.rng.Perm(len(cands))[:sampleLimit] {
			sample[i] = cands[j]
		}
	}
	buckets := map[string]int{}
	for _, a := range sample {
		buckets[Score(g, a)]++
	}
	sum := 0
	for pattern, n := range buckets {
		if pattern != Solved {
			sum += n * n
		}
	}
	return float64(sum) / float64(len(sample))
}

// nearMiss picks one of the unguessed common words that come closest to
// fitting every clue without being the answer, the kind of guess a person
// makes when torn between similar words.
func (s *Solver) nearMiss(answer string) (string, bool) {
	guessed := map[string]bool{}
	for _, t := range s.History {
		guessed[t.Guess] = true
	}
	fit := map[string]int{}
	var options []string
	for _, w := range s.words {
		if w == answer || guessed[w] {
			continue
		}
		for _, t := range s.History {
			got := Score(t.Guess, w)
			for i := range 5 {
				if got[i] == t.Pattern[i] {
					fit[w]++
				}
			}
		}
		options = append(options, w)
	}
	if len(options) == 0 {
		return "", false
	}
	slices.SortStableFunc(options, func(a, b string) int { return cmp.Compare(fit[b], fit[a]) })
	pool := options[:min(4, len(options))]
	return pool[s.rng.IntN(len(pool))], true
}

// Play runs a full game against a known answer. Newer NYT answers are
// missing from the original list, so the answer is added when absent.
func Play(answer string, rng *rand.Rand, target, limit int) []Turn {
	s := NewSolver(rng, target, limit)
	if !slices.Contains(s.words, answer) {
		s.words = append(slices.Clone(s.words), answer)
	}
	return playWith(s, answer)
}

func playWith(s *Solver, answer string) []Turn {
	for s.Turn() <= 6 {
		guess, err := s.NextGuess()
		if err != nil {
			break
		}
		pattern := Score(guess, answer)
		s.Record(guess, pattern)
		if pattern == Solved {
			break
		}
	}
	return s.History
}
