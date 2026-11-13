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
	// Opener, when set, is the first guess, unless careful play needs a
	// stronger one.
	Opener string
	// Hard keeps every guess legal in Hard Mode (see HardValid).
	Hard     bool
	History  []Turn
	words    []string        // possible answers
	excluded map[string]bool // ruled out unless nothing else fits
	rng      *rand.Rand
}

var Openers = []string{
	"adieu", "audio", "crane", "slate", "stare", "raise", "arise", "house",
	"train", "roast", "irate", "later", "trace", "crate", "stone", "heart",
	"great", "least", "about", "spare", "react", "alert", "steam", "plane",
}

// StrongOpeners are popular openers that split the answers well enough for
// tight limits;
// under -max 4 weaker favourites like ADIEU miss about 1 game in 10.
var StrongOpeners = []string{
	"stare", "slate", "roast", "heart", "trace", "steam", "spare", "react",
	"later", "crane", "train",
}

// finishWeights is roughly how often a regular player finishes on each turn.
var finishWeights = map[int]float64{2: 0.04, 3: 0.27, 4: 0.40, 5: 0.22, 6: 0.07}

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
	return &Solver{Target: min(target, limit), Max: limit, Sloppiness: 0.2, words: Answers, rng: rng}
}

// drawTarget samples finishWeights, cut off at limit.
func drawTarget(rng *rand.Rand, limit int) int {
	total := 0.0
	for turn := 2; turn <= limit; turn++ {
		total += finishWeights[turn]
	}
	x := rng.Float64() * total
	for turn := 2; turn <= limit; turn++ {
		if x < finishWeights[turn] {
			return turn
		}
		x -= finishWeights[turn]
	}
	return limit
}

func (s *Solver) Turn() int { return len(s.History) + 1 }

// Candidates are the possible answers that fit every clue so far, leaving
// out excluded words unless nothing else fits.
func (s *Solver) Candidates() []string {
	all := Filter(s.words, s.History)
	if len(s.excluded) == 0 {
		return all
	}
	var rest []string
	for _, w := range all {
		if !s.excluded[w] {
			rest = append(rest, w)
		}
	}
	if len(rest) == 0 {
		return all
	}
	return rest
}

// Exclude rules words out as answers, as a player who knows NYT's past
// answers does, while still allowing them as guesses. If the clues leave
// nothing else, the excluded words come back, so a repeated answer is found.
func (s *Solver) Exclude(words ...string) {
	if s.excluded == nil {
		s.excluded = map[string]bool{}
	}
	for _, w := range words {
		s.excluded[w] = true
	}
}

func (s *Solver) Record(guess, pattern string) {
	s.History = append(s.History, Turn{guess, pattern})
}

func (s *Solver) NextGuess() (string, error) {
	if len(s.History) == 0 {
		if s.Opener != "" && (!s.careful() || slices.Contains(StrongOpeners, s.Opener)) {
			return s.Opener, nil
		}
		openers := Openers
		if s.careful() {
			openers = StrongOpeners
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
		return s.choose(cands)
	case 2:
		g, _ := s.bestForTwo(cands)
		return g
	}
	// With four or more of six guesses left a slip costs little and can't
	// lose. A tighter limit has no room for one (a slip on guess 2 under a
	// limit of 5 can miss it), so the solver then plays carefully.
	if !s.careful() && turnsLeft >= 4 && s.rng.Float64() < s.Sloppiness {
		if w, ok := s.slip(cands); ok {
			return w
		}
	}
	ranked := s.rank(cands, cands)
	if turnsLeft == 3 && len(cands) > 3 {
		return s.bestForThree(cands, ranked)
	}
	if s.careful() {
		return s.sharpest(cands, ranked)
	}

	if ahead && turnsLeft >= 3 && len(cands) <= 12 {
		// Ahead of pace with few words left: spend a turn on an elimination
		// word, as people do, which also can't win too early.
		if pool := s.eliminators(cands, 5); len(pool) > 0 {
			return s.choose(pool)
		}
	}
	if len(cands) > 2 && len(cands) >= turnsLeft {
		// A trap like _OUND: guessing one fitting word per turn could lose,
		// so take an elimination word unless a fitting word splits as well.
		return s.sharpest(cands, ranked)
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
	return s.choose(pool)
}

// choose picks from words at random, favouring familiar ones the way a
// person thinks of HOUSE long before NADIR.
func (s *Solver) choose(words []string) string {
	total := 0.0
	weights := make([]float64, len(words))
	for i, w := range words {
		weights[i] = familiarity(w)
		total += weights[i]
	}
	x := s.rng.Float64() * total
	for i, wt := range weights {
		if x < wt {
			return words[i]
		}
		x -= wt
	}
	return words[len(words)-1]
}

// allowed reports whether w may be guessed now; only Hard Mode forbids any.
func (s *Solver) allowed(w string) bool { return !s.Hard || HardValid(w, s.History) }

// careful reports whether there is no room for human detours: under a
// guess limit below six, or in Hard Mode, where traps like _ATCH can only be
// avoided early. The solver then opens strongly and plays its sharpest guess
// every turn.
func (s *Solver) careful() bool { return s.Max < 6 || s.Hard }

// sharpest is the best fitting word, or a better-splitting elimination word.
func (s *Solver) sharpest(cands, ranked []string) string {
	best := ranked[0]
	if pool := s.eliminators(cands, 1); len(pool) > 0 &&
		s.expectedLeft(pool[0], cands) < s.expectedLeft(best, cands) {
		return pool[0]
	}
	return best
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
		if guessed[w] || !s.allowed(w) {
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
	return s.choose(pool), true
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
			if !fits[g] && bestWins < len(cands) && s.allowed(g) {
				consider(g)
			}
		}
	}
	return best, bestWins
}

// singletons counts the answers in cands that g pins down to one word,
// including g itself when it fits.
func singletons(g string, cands []string) int {
	// Small inputs are the common case inside the lookahead, so compare
	// codes pairwise rather than clearing a full 243-entry table.
	var small [16]int
	if len(cands) <= len(small) {
		codes := small[:len(cands)]
		for i, a := range cands {
			codes[i] = scoreCode(g, a)
		}
		n := 0
		for i, c := range codes {
			unique := true
			for j, d := range codes {
				if i != j && c == d {
					unique = false
					break
				}
			}
			if unique {
				n++
			}
		}
		return n
	}
	var buckets [243]int
	for _, a := range cands {
		buckets[scoreCode(g, a)]++
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
		buckets := map[int][]string{}
		for _, a := range cands {
			p := scoreCode(g, a)
			buckets[p] = append(buckets[p], a)
		}
		wins := 0
		for p, bucket := range buckets {
			if p == solvedCode || len(bucket) <= 2 {
				// Guessing either of two words is sure to solve both.
				wins += len(bucket)
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
		if !fits[w] && s.allowed(w) {
			others = append(others, w)
		}
	}
	ranked := s.rank(others, cands)
	return ranked[:min(n, len(ranked))]
}

// rank orders guesses by how few of cands they would leave on average.
func (s *Solver) rank(guesses, cands []string) []string {
	type scored struct {
		word  string
		score float64
	}
	all := make([]scored, len(guesses))
	for i, g := range guesses {
		all[i] = scored{g, s.expectedLeft(g, cands)}
	}
	slices.SortStableFunc(all, func(a, b scored) int { return cmp.Compare(a.score, b.score) })
	ranked := make([]string, len(all))
	for i, sc := range all {
		ranked[i] = sc.word
	}
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
	var buckets [243]int
	for _, a := range sample {
		buckets[scoreCode(g, a)]++
	}
	sum := 0
	for code, n := range buckets {
		if code != solvedCode {
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
		if w == answer || guessed[w] || !s.allowed(w) {
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
	return s.choose(pool), true
}

// AddWords adds possible answers missing from the original list, such as
// newer NYT answers.
func (s *Solver) AddWords(words ...string) {
	cloned := false
	for _, w := range words {
		if slices.Contains(s.words, w) {
			continue
		}
		if !cloned {
			s.words, cloned = slices.Clone(s.words), true
		}
		s.words = append(s.words, w)
	}
}

// Play runs a full game against a known answer with a default solver.
func Play(answer string, rng *rand.Rand, target, limit int) []Turn {
	return NewSolver(rng, target, limit).Solve(answer)
}

// Solve plays a full game against a known answer, using only the clues each
// guess earns. The answer is added to the word list if it is missing.
func (s *Solver) Solve(answer string) []Turn {
	s.AddWords(answer)
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
