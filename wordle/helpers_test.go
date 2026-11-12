package wordle

import (
	"math/rand/v2"
	"sync"
)

// streams is fixed, not tied to the CPU count, so every machine plays the
// same games for a given seed.
const streams = 8

// playAll plays every answer with solvers from newSolver, in parallel.
// Answer i always goes to stream i%streams, whose generator is seeded from
// seed and the stream number, so results are reproducible anywhere.
func playAll(answers []string, seed uint64, newSolver func(*rand.Rand) *Solver) [][]Turn {
	out := make([][]Turn, len(answers))
	var wg sync.WaitGroup
	for st := range streams {
		wg.Go(func() {
			rng := rand.New(rand.NewPCG(seed, uint64(st)))
			for i := st; i < len(answers); i += streams {
				out[i] = newSolver(rng).Solve(answers[i])
			}
		})
	}
	wg.Wait()
	return out
}
