// Package eval runs six-turn model games without retries.
package eval

import (
	"encoding/json"
	"fmt"
	"github.com/notzenco/wordle-solver/internal/wordle"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"
)

type Reply struct {
	Text   string
	Model  string
	Tokens int
}
type Observation struct {
	History  []wordle.Turn
	Attempts int
	Hard     bool
}
type Player interface {
	Guess(Observation) (Reply, error)
}
type Scripted struct {
	Replies []Reply
	index   int
}

func (p *Scripted) Guess(o Observation) (Reply, error) {
	if p.index >= len(p.Replies) {
		return Reply{}, fmt.Errorf("script exhausted")
	}
	r := p.Replies[p.index]
	p.index++
	return r, nil
}
func SplitModel(s string) (string, string, error) {
	p, m, ok := strings.Cut(s, "/")
	if !ok || p == "" || m == "" {
		return "", "", fmt.Errorf("model must be provider/model")
	}
	return p, m, nil
}

var word = regexp.MustCompile(`^[a-z]{5}$`)

func Parse(s string) (string, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{") {
		var r struct {
			Guess string `json:"guess"`
		}
		if json.Unmarshal([]byte(s), &r) != nil {
			return "", fmt.Errorf("malformed reply")
		}
		s = r.Guess
	}
	s = strings.ToLower(strings.Trim(strings.TrimSpace(s), "`\"'"))
	if !word.MatchString(s) {
		return "", fmt.Errorf("reply must contain exactly one five-letter word")
	}
	return s, nil
}
func Sample(set string, n int, seed uint64) []string {
	pool := wordle.Answers
	if set == "heldout" {
		pool = nil
		for _, w := range wordle.ValidWords() {
			if !wordle.InAnswers(w) {
				pool = append(pool, w)
			}
		}
	}
	rng := rand.New(rand.NewPCG(seed, seed))
	perm := rng.Perm(len(pool))
	out := []string{}
	for _, i := range perm[:min(n, len(perm))] {
		out = append(out, pool[i])
	}
	return out
}

type Attempt struct {
	Reply    Reply
	Guess    string
	Pattern  string
	Problem  string
	Duration time.Duration
}
type Result struct {
	Answer           string
	Solved           bool
	Guesses          int
	Baseline         int
	BaselineSolved   bool
	Beyond           int
	BaselineStrategy string
	Invalid          int
	Malformed        int
	HardViolations   int
	Tokens           int
	Duration         time.Duration
	Rerouted         bool
	Attempts         []Attempt
}

func Play(p Player, answer, model string, hard bool) Result {
	start := time.Now()
	r := Result{Answer: answer, BaselineStrategy: "entropy"}
	st, _ := wordle.StrategyByName("entropy")
	pool := wordle.Answers
	if !wordle.InAnswers(answer) {
		pool = Sample("heldout", 100000, 0)
	}
	baseline := st.PlayPool(answer, hard, pool)
	r.Baseline = len(baseline)
	r.BaselineSolved = len(baseline) > 0 && baseline[len(baseline)-1].Pattern == wordle.Solved
	valid := map[string]bool{}
	for _, w := range wordle.ValidWords() {
		valid[w] = true
	}
	history := []wordle.Turn{}
	for turn := 0; turn < 6; turn++ {
		t := time.Now()
		reply, err := p.Guess(Observation{append([]wordle.Turn(nil), history...), turn, hard})
		a := Attempt{Reply: reply, Duration: time.Since(t)}
		r.Tokens += reply.Tokens
		r.Guesses++
		if reply.Model != "" && reply.Model != model {
			r.Rerouted = true
		}
		guess, e := Parse(reply.Text)
		a.Guess = guess
		switch {
		case err != nil:
			a.Problem = "request failed"
		case e != nil:
			r.Malformed++
			a.Problem = "malformed"
		case !valid[guess]:
			r.Invalid++
			a.Problem = "invalid word"
		case hard && !wordle.HardValid(guess, history):
			r.HardViolations++
			a.Problem = "hard mode violation"
		default:
			a.Pattern = wordle.Score(guess, answer)
			history = append(history, wordle.Turn{Guess: guess, Pattern: a.Pattern})
			r.Solved = a.Pattern == wordle.Solved
		}
		r.Attempts = append(r.Attempts, a)
		if r.Solved {
			break
		}
	}
	r.Beyond = r.Guesses - r.Baseline
	r.Duration = time.Since(start)
	return r
}
