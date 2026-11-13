package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	modeleval "github.com/notzenco/wordle-solver/internal/eval"
)

func gameState(o modeleval.Observation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Play Wordle: find a five-letter English word in six attempts. Feedback colours: green (g) means correct position, yellow (y) means elsewhere, grey (b) means absent. Repeated letters consume remaining answer occurrences. Hard Mode: %t; keep green positions and reuse every revealed letter with its multiplicity. Attempts used: %d.\n", o.Hard, o.Attempts)
	for _, t := range o.History {
		fmt.Fprintf(&b, "%s: ", strings.ToUpper(t.Guess))
		for i, c := range t.Pattern {
			colour := map[rune]string{'g': "green", 'y': "yellow", 'b': "grey"}[c]
			fmt.Fprintf(&b, "%c=%s ", strings.ToUpper(t.Guess)[i], colour)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

type systemonePlayer struct {
	provider provider
	model    string
	timeout  time.Duration
}

func (p *systemonePlayer) Guess(o modeleval.Observation) (modeleval.Reply, error) {
	if len(o.Options) == 0 {
		return modeleval.Reply{}, fmt.Errorf("systemone requires candidate options")
	}
	criteria := map[string]string{}
	for _, w := range o.Options {
		criteria[w] = strings.ToUpper(w)
	}
	body, _ := json.Marshal(map[string]any{"model": p.model, "state": gameState(o), "truncate": false, "questions": map[string]any{"guess": map[string]any{"type": "choice", "instructions": "Choose the next Wordle guess. Use the rules and colour history; avoid inconsistent words.", "criteria": criteria}}})
	b, e := apiRequest(&http.Client{Timeout: p.timeout}, p.provider, "/systemone", bytes.NewReader(body))
	if e != nil {
		return modeleval.Reply{}, e
	}
	var r struct {
		Model   string
		Answers struct {
			Guess struct {
				Choice        string
				Confidence    float64
				Probabilities map[string]float64
			}
		}
		Usage struct {
			InputTokens int `json:"input_tokens"`
		}
	}
	if json.Unmarshal(b, &r) != nil {
		return modeleval.Reply{}, fmt.Errorf("invalid SystemOne response")
	}
	g := r.Answers.Guess
	if _, ok := criteria[g.Choice]; !ok {
		return modeleval.Reply{}, fmt.Errorf("SystemOne returned an unoffered choice")
	}
	sum := 0.0
	for _, w := range o.Options {
		v, ok := g.Probabilities[w]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return modeleval.Reply{}, fmt.Errorf("invalid SystemOne probabilities")
		}
		sum += v
	}
	if math.Abs(sum-1) > 0.01 || len(g.Probabilities) != len(criteria) {
		return modeleval.Reply{}, fmt.Errorf("invalid SystemOne probability distribution")
	}
	return modeleval.Reply{Text: g.Choice, Model: r.Model, Tokens: r.Usage.InputTokens, Confidence: g.Confidence, Probabilities: g.Probabilities}, nil
}
