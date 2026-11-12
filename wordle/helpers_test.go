package wordle

import (
	"math/rand/v2"
	"runtime"
	"sync"
)

// playAll plays every answer with solvers from newSolver, spread over the
// CPUs. Answer i always goes to worker i%workers, whose generator is seeded
// from seed and the worker number, so results are reproducible on a given
// machine.
func playAll(answers []string, seed uint64, newSolver func(*rand.Rand) *Solver) [][]Turn {
	workers := runtime.GOMAXPROCS(0)
	out := make([][]Turn, len(answers))
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			rng := rand.New(rand.NewPCG(seed, uint64(w)))
			for i := w; i < len(answers); i += workers {
				out[i] = newSolver(rng).Solve(answers[i])
			}
		})
	}
	wg.Wait()
	return out
}
