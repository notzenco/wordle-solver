package eval

import (
	"math"
	"math/rand/v2"
	"slices"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

// Options samples without looking at the true answer. Distractors are added
// after capping consistent words; the final shuffle hides their positions.
func Options(history []wordle.Turn, cap, distractors int, seed uint64) []string {
	valid := wordle.ValidWords()
	fitting := wordle.Filter(valid, history)
	rng := rand.New(rand.NewPCG(seed, seed))
	sample := func(pool []string, n int) []string {
		perm := rng.Perm(len(pool))
		out := []string{}
		for _, i := range perm[:min(n, len(pool))] {
			out = append(out, pool[i])
		}
		return out
	}
	out := sample(fitting, cap)
	fittingSet := map[string]bool{}
	for _, w := range fitting {
		fittingSet[w] = true
	}
	inconsistent := []string{}
	for _, w := range valid {
		if !fittingSet[w] {
			inconsistent = append(inconsistent, w)
		}
	}
	out = append(out, sample(inconsistent, distractors)...)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

type Metrics struct {
	TrueProbability  float64
	LogLoss          float64
	UniformLogLoss   float64
	FrequencyLogLoss float64
}

// Measure uses natural logs and a 1e-15 floor for zero model probability.
func Measure(answer string, options []string, probabilities map[string]float64) *Metrics {
	if !slices.Contains(options, answer) || probabilities == nil {
		return nil
	}
	p, ok := probabilities[answer]
	if !ok {
		return nil
	}
	sum := 0.0
	for _, w := range options {
		sum += wordle.FrequencyWeight(w)
	}
	return &Metrics{p, -math.Log(math.Max(p, 1e-15)), math.Log(float64(len(options))), -math.Log(wordle.FrequencyWeight(answer) / sum)}
}

type PlayOptions struct {
	Candidates  bool
	MaxOptions  int
	Distractors int
	Seed        uint64
	// Retries lets a rejected word (malformed, not in the word list, or
	// breaking Hard Mode) be replaced without using a turn, up to this many
	// times per turn, as the game itself rejects such words. Eval scoring
	// leaves it at zero so every reply counts.
	Retries int
}
