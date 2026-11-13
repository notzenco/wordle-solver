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

func TestParseAcceptsFencedJSON(t *testing.T) {
	for _, reply := range []string{"```json\n{\n  \"guess\": \"AUDIO\"\n}\n```", "```\n{\"guess\":\"audio\"}\n```", "`audio`"} {
		if g, err := Parse(reply); err != nil || g != "audio" {
			t.Errorf("Parse(%q) = %q, %v", reply, g, err)
		}
	}
	if _, err := Parse("```json\n{\"guess\": \"audio\", \"also\": \"crane\"}\n``` and crane"); err == nil {
		t.Error("text outside the fence was accepted")
	}
}

func TestRetriesLetRejectedWordsSkipTheTurnCount(t *testing.T) {
	replies := func() *Scripted {
		return &Scripted{Replies: []Reply{{Text: "zzzzz"}, {Text: "toolong"}, {Text: "crane"}, {Text: "crazy"}}}
	}
	strict := PlayWithOptions(replies(), "crazy", "m", false, PlayOptions{})
	if strict.Guesses != 4 || strict.Invalid != 1 || strict.Malformed != 1 {
		t.Fatalf("strict scoring: %+v", strict)
	}
	game := PlayWithOptions(replies(), "crazy", "m", false, PlayOptions{Retries: 3})
	if !game.Solved || game.Guesses != 2 || len(game.Attempts) != 4 || !game.Attempts[0].Free || game.Attempts[2].Free {
		t.Fatalf("with retries: solved %v, guesses %d, attempts %+v", game.Solved, game.Guesses, game.Attempts)
	}
	capped := PlayWithOptions(&Scripted{Replies: []Reply{{Text: "zzzzz"}, {Text: "zzzzz"}, {Text: "crazy"}}}, "crazy", "m", false, PlayOptions{Retries: 1})
	if capped.Guesses != 2 {
		t.Fatalf("one retry should let the second rejection use a turn: %d", capped.Guesses)
	}
}
