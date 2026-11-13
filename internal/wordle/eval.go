package wordle

import "sort"

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

// FrequencyWeight is the prior for the eval's frequency baseline: the same
// familiarity score the solver uses. Raw subtitle counts are so skewed that a
// few very common words take nearly all the weight, which scored worse than
// a uniform guess over the answers.
func FrequencyWeight(w string) float64 { return familiarity(w) }
