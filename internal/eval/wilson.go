package eval

import "math"

// Wilson is the 95% Wilson score interval for k successes in n trials,
// which stays inside 0–1 and behaves at 0 and n, unlike the normal one.
func Wilson(k, n int) (lo, hi float64) {
	if n == 0 {
		return 0, 1
	}
	const z = 1.959964
	p, nf := float64(k)/float64(n), float64(n)
	centre := (p + z*z/(2*nf)) / (1 + z*z/nf)
	half := z / (1 + z*z/nf) * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf))
	return max(0, centre-half), min(1, centre+half)
}
