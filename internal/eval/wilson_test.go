package eval

import (
	"math"
	"testing"
)

func TestWilsonInterval(t *testing.T) {
	for _, c := range []struct {
		k, n   int
		lo, hi float64
	}{
		{48, 50, 0.865, 0.989},
		{50, 50, 0.929, 1},
		{0, 10, 0, 0.278},
	} {
		lo, hi := Wilson(c.k, c.n)
		if math.Abs(lo-c.lo) > 0.001 || math.Abs(hi-c.hi) > 0.001 {
			t.Errorf("Wilson(%d, %d) = %.4f–%.4f, want %.3f–%.3f", c.k, c.n, lo, hi, c.lo, c.hi)
		}
	}
	if lo, hi := Wilson(0, 0); lo != 0 || hi != 1 {
		t.Errorf("no games should give the whole range, got %v–%v", lo, hi)
	}
}
