package eval

import (
	"errors"
	"fmt"
	"github.com/notzenco/wordle-solver/internal/wordle"
	"math"
	"slices"
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

// failing returns err on every call, with whatever metadata reply carries.
type failing struct {
	reply Reply
	err   error
}

func (p failing) Guess(Observation) (Reply, error) { return p.reply, p.err }

func TestRequestFailuresAreCountedWithTheirCause(t *testing.T) {
	r := Play(failing{err: errors.New("connection refused")}, "cigar", "m", false)
	if r.RequestFailures != 6 || r.Guesses != 6 || r.Solved {
		t.Fatalf("%+v", r)
	}
	if a := r.Attempts[0]; a.Problem != "request failed" || a.Error != "connection refused" {
		t.Fatalf("attempt %+v", a)
	}
}

func TestMalformedRepliesKeepTheirModelAndTokens(t *testing.T) {
	p := failing{reply: Reply{Model: "other", Tokens: 7}, err: fmt.Errorf("%w: unoffered choice", ErrMalformed)}
	r := Play(p, "cigar", "wanted", false)
	if !r.Rerouted || r.Tokens != 42 || r.Malformed != 6 || r.RequestFailures != 0 {
		t.Fatalf("%+v", r)
	}
	if a := r.Attempts[0]; a.Problem != "malformed" || a.Error == "" {
		t.Fatalf("attempt %+v", a)
	}
}

func TestSameModelAllowsTagAndSnapshotAliases(t *testing.T) {
	for _, c := range []struct {
		requested, reported string
		same                bool
	}{
		{"llama3.2", "llama3.2", true},
		{"llama3.2", "llama3.2:latest", true},
		{"gpt-4o", "gpt-4o-2024-08-06", true},
		{"claude-x", "claude-x-20250101", true},
		{"gpt-4o", "gpt-4o-mini", false},
		{"llama3.2", "llama3.2:70b", false},
		{"gpt-4o", "gpt-4o-2024-08-06-extra", false},
		{"clef", "other", false},
	} {
		if got := SameModel(c.requested, c.reported); got != c.same {
			t.Errorf("SameModel(%q, %q) = %v", c.requested, c.reported, got)
		}
	}
}

// The matched set pairs each sampled answer with a held-out word about as
// familiar, so it tests unfamiliarity to the model, not obscurity.
func TestMatchedSetIsHeldOutButAsFamiliar(t *testing.T) {
	answers := Sample("answers", 200, 42)
	matched := Sample("matched", 200, 42)
	heldout := Sample("heldout", 200, 42)
	seen := map[string]bool{}
	for _, w := range matched {
		if wordle.InAnswers(w) || seen[w] {
			t.Fatalf("%s is an answer or repeated", w)
		}
		seen[w] = true
	}
	median := func(ws []string) float64 {
		f := make([]float64, len(ws))
		for i, w := range ws {
			f[i] = wordle.FrequencyWeight(w)
		}
		slices.Sort(f)
		return f[len(f)/2]
	}
	a, m, h := median(answers), median(matched), median(heldout)
	if math.Abs(m-a) > math.Abs(h-a)/4 {
		t.Fatalf("median familiarity: answers %.0f, matched %.0f, held-out %.0f", a, m, h)
	}
	if again := Sample("matched", 200, 42); !slices.Equal(again, matched) {
		t.Fatal("matched sample is not reproducible")
	}
}
