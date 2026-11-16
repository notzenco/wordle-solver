package eval

import (
	"github.com/notzenco/wordle-solver/internal/wordle"
	"math"
	"reflect"
	"slices"
	"testing"
)

func TestOptions(t *testing.T) {
	h := []wordle.Turn{{Guess: "raise", Pattern: wordle.Score("raise", "cigar")}}
	a := Options(h, 4, 2, 42)
	b := Options(h, 4, 2, 42)
	if !reflect.DeepEqual(a, b) || len(a) != 6 {
		t.Fatal(a, b)
	}
	consistent := 0
	for _, w := range a {
		if len(wordle.Filter([]string{w}, h)) == 1 {
			consistent++
		}
	}
	if consistent != 4 {
		t.Fatal(consistent, a)
	}
}
func TestProbabilityMetrics(t *testing.T) {
	m := Measure("cigar", []string{"cigar", "raise"}, map[string]float64{"cigar": 0.25, "raise": 0.75})
	if m == nil || m.TrueProbability != 0.25 || math.Abs(m.LogLoss-math.Log(4)) > 1e-9 || math.Abs(m.UniformLogLoss-math.Log(2)) > 1e-9 {
		t.Fatal(m)
	}
	if Measure("cigar", []string{"raise"}, map[string]float64{"raise": 1}) != nil {
		t.Fatal("answer absent")
	}
}

func TestFrequencyBaselineAndZeroProbability(t *testing.T) {
	options := []string{"cigar", "raise"}
	m := Measure("cigar", options, map[string]float64{"cigar": 0, "raise": 1})
	want := -math.Log(wordle.FrequencyWeight("cigar") / (wordle.FrequencyWeight("cigar") + wordle.FrequencyWeight("raise")))
	if math.Abs(m.FrequencyLogLoss-want) > 1e-12 || math.Abs(m.LogLoss-(-math.Log(1e-15))) > 1e-12 {
		t.Fatal(m)
	}
}
func TestOptionsEmptyHistory(t *testing.T) {
	a := Options(nil, 64, 5, 42)
	if len(a) != 64 {
		t.Fatal("no inconsistent words exist without clues")
	}
	if reflect.DeepEqual(a, Options(nil, 64, 5, 43)) {
		t.Fatal("seed ignored")
	}
}

// Answers are mostly common words, so a frequency prior should beat
// guessing uniformly among the offered options.
func TestFrequencyBaselineBeatsUniformOnAnswers(t *testing.T) {
	answers := Sample("answers", 300, 7)
	var freq, uniform float64
	for i, a := range answers {
		opts := append(Options(nil, 63, 0, uint64(i)), a)
		probs := map[string]float64{a: 1}
		m := Measure(a, opts, probs)
		freq += m.FrequencyLogLoss
		uniform += m.UniformLogLoss
	}
	if freq >= uniform {
		t.Fatalf("frequency baseline %.3f is no better than uniform %.3f", freq/300, uniform/300)
	}
}

// recorder answers with its first offered word and keeps what it was offered.
type recorder struct{ offered [][]string }

func (r *recorder) Guess(o Observation) (Reply, error) {
	r.offered = append(r.offered, o.Options)
	return Reply{Text: o.Options[0]}, nil
}

// Each game must get its own options: with one seed for every game, every
// run opened on the same 64 words and the answer was almost never offered.
func TestEachGameGetsItsOwnOptions(t *testing.T) {
	first := func(game int) []string {
		r := &recorder{}
		PlayWithOptions(r, "cigar", "m", false, PlayOptions{Candidates: true, MaxOptions: 64, Seed: 42, Game: game})
		return r.offered[0]
	}
	if a, b := first(0), first(1); slices.Equal(a, b) {
		t.Fatal("games 0 and 1 were offered the same opening words")
	}
	if a, b := first(3), first(3); !slices.Equal(a, b) {
		t.Fatal("the same seed and game must repeat")
	}
}
