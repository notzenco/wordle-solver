package wordle

import (
	"hash/fnv"
	"sort"
	"sync"
	"sync/atomic"
)

// ValidWords includes every frequency word and every original answer.
func ValidWords() []string {
	m := map[string]bool{}
	for w := range frequency {
		m[w] = true
	}
	for _, w := range Answers {
		m[w] = true
	}
	out := make([]string, 0, len(m))
	for w := range m {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// IsValid reports whether w is in ValidWords.
func IsValid(w string) bool { _, ok := frequency[w]; return ok || InAnswers(w) }

// PlayPool applies entropy's scoring rule to an explicit candidate pool.
// The ordinary answer benchmark continues to use the precomputed table.
func (st Strategy) PlayPool(answer string, hard bool, pool []string) []Turn {
	if InAnswers(answer) {
		return st.Play(answer, hard)
	}
	if st.Name != "entropy" {
		panic("explicit pools require entropy")
	}
	cands := append([]string(nil), pool...)
	history := []Turn{}
	for len(history) < 6 && len(cands) > 0 {
		var best string
		if len(history) == 0 {
			best = poolOpening(cands, hard)
		} else {
			best = bestInPool(cands, history, hard)
		}
		if best == "" {
			break
		}
		pattern := Score(best, answer)
		history = append(history, Turn{best, pattern})
		if pattern == Solved {
			break
		}
		cands = Filter(cands, history)
	}
	return history
}

func bestInPool(cands []string, history []Turn, hard bool) string {
	best := ""
	bestScore := -1.0
	for _, g := range cands {
		if hard && !HardValid(g, history) {
			continue
		}
		var buckets [243]int
		for _, a := range cands {
			buckets[scoreCode(g, a)]++
		}
		h := entropy(&buckets, len(cands))
		if h > bestScore {
			best, bestScore = g, h
		}
	}
	return best
}

var (
	openings     sync.Map // pool hash and mode -> first guess
	poolOpenings atomic.Int64
)

// poolOpening is the first guess for pool, cached by its contents and order
// (order breaks ties) because every game over the same pool opens the same way.
func poolOpening(pool []string, hard bool) string {
	h := fnv.New64a()
	for _, w := range pool {
		h.Write([]byte(w))
	}
	key := [2]uint64{h.Sum64(), 0}
	if hard {
		key[1] = 1
	}
	if g, ok := openings.Load(key); ok {
		return g.(string)
	}
	poolOpenings.Add(1)
	g := bestInPool(pool, nil, hard)
	openings.Store(key, g)
	return g
}

// FrequencyWeight is the prior for the eval's frequency baseline: the same
// familiarity score the solver uses. Raw subtitle counts are so skewed that a
// few very common words take nearly all the weight, which scored worse than
// a uniform guess over the answers.
func FrequencyWeight(w string) float64 { return familiarity(w) }
