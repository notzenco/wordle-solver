package eval

import (
	"github.com/notzenco/wordle-solver/internal/wordle"
	"testing"
)

func TestSplit(t *testing.T) {
	p, m, e := SplitModel("omlx/mlx-community/a")
	if e != nil || p != "omlx" || m != "mlx-community/a" {
		t.Fatal(p, m, e)
	}
}
func TestParse(t *testing.T) {
	for _, s := range []string{"CRANE", `{"guess":"crane"}`, " `crane` "} {
		w, e := Parse(s)
		if e != nil || w != "crane" {
			t.Fatal(s, w, e)
		}
	}
	if _, e := Parse("crane or raise"); e == nil {
		t.Fatal("ambiguous")
	}
}
func TestGame(t *testing.T) {
	p := &Scripted{Replies: []Reply{{Text: "??"}, {Text: "zzzzz"}, {Text: "raise"}, {Text: "cigar", Model: "other"}}}
	r := Play(p, "cigar", "wanted", true)
	if r.Malformed != 1 || r.Invalid != 1 || !r.Rerouted || !r.Solved || r.Guesses != 4 {
		t.Fatalf("%+v", r)
	}
	p = &Scripted{Replies: []Reply{{Text: "cigar"}, {Text: "raise"}}}
	r = Play(p, "cider", "", true)
	if r.HardViolations < 1 {
		t.Fatal(r)
	}
	a := Sample("heldout", 3, 42)
	b := Sample("heldout", 3, 42)
	for i := range a {
		if a[i] != b[i] || wordle.InAnswers(a[i]) {
			t.Fatal(a, b)
		}
	}
	r = Play(&Scripted{Replies: []Reply{{Text: "raise"}}}, "raise", "", false)
	if r.Beyond != 0 || r.Baseline != 1 {
		t.Fatal(r)
	}
}
