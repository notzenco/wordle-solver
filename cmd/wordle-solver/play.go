package main

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"time"

	modeleval "github.com/notzenco/wordle-solver/internal/eval"
	"github.com/notzenco/wordle-solver/internal/wordle"
)

// modelSuggester asks a chat or decision model for each guess. Decision
// models choose from up to maxOptions words that fit the clues so far.
type modelSuggester struct {
	provider   provider
	model      string
	kind       string
	autoKind   bool
	maxOptions int
	timeout    time.Duration
	rng        *rand.Rand
	valid      map[string]bool
}

func newModelSuggester(p provider, model string, rng *rand.Rand) *modelSuggester {
	valid := map[string]bool{}
	for _, w := range wordle.ValidWords() {
		valid[w] = true
	}
	return &modelSuggester{provider: p, model: model, kind: providerKind(p), autoKind: p.Kind == "",
		maxOptions: 64, timeout: time.Minute, rng: rng, valid: valid}
}

// suggest returns the model's guess and a short description of how it was
// chosen, or an error saying why the reply can't be used.
func (m *modelSuggester) suggest(history []wordle.Turn, hard bool) (string, string, error) {
	for {
		var offered []string
		var player modeleval.Player = &chatPlayer{m.provider, m.model, 0, 64, m.timeout}
		if m.kind == "systemone" {
			offered = modeleval.Options(history, m.maxOptions, 0, m.rng.Uint64())
			player = &systemonePlayer{m.provider, m.model, m.timeout}
		}
		reply, err := player.Guess(modeleval.Observation{History: history, Attempts: len(history), Hard: hard, Options: offered})
		if errors.Is(err, modeleval.ErrDecisionModel) && m.autoKind {
			fmt.Fprintf(stderr, "%s is a decision model; it will choose from the words that fit the clues.\n", m.model)
			m.kind, m.autoKind = "systemone", false
			continue
		}
		if err != nil {
			return "", "", err
		}
		guess, err := modeleval.Parse(reply.Text)
		switch {
		case err != nil:
			return "", "", fmt.Errorf("unreadable reply %q", reply.Text)
		case !m.valid[guess]:
			return "", "", fmt.Errorf("%s is not a valid word", strings.ToUpper(guess))
		case hard && !wordle.HardValid(guess, history):
			return "", "", fmt.Errorf("%s breaks Hard Mode", strings.ToUpper(guess))
		}
		if offered == nil {
			return guess, m.model, nil
		}
		if len(offered) == 1 {
			return guess, fmt.Sprintf("%s, the only word that fits", m.model), nil
		}
		return guess, fmt.Sprintf("%s, %s of %d options", m.model, topChoices(reply.Probabilities, 3), len(offered)), nil
	}
}

// topChoices formats the most probable options, such as "39% CRANE, 12% SLATE".
func topChoices(p map[string]float64, n int) string {
	words := make([]string, 0, len(p))
	for w := range p {
		words = append(words, w)
	}
	sort.Slice(words, func(i, j int) bool {
		if p[words[i]] != p[words[j]] {
			return p[words[i]] > p[words[j]]
		}
		return words[i] < words[j]
	})
	parts := []string{}
	for _, w := range words[:min(n, len(words))] {
		parts = append(parts, fmt.Sprintf("%.0f%% %s", 100*p[w], strings.ToUpper(w)))
	}
	return strings.Join(parts, ", ")
}

func play(s *wordle.Solver, opts options, m *modelSuggester) {
	var in *bufio.Scanner
	if stdin != nil {
		in = bufio.NewScanner(stdin)
	} else {
		in = bufio.NewScanner(os.Stdin)
	}
	fmt.Fprintln(stdout, "Type each suggestion into Wordle, then enter the colours it showed")
	fmt.Fprintln(stdout, "as g/y/b (e.g. bygbb). To play your own word instead: 'word pattern'.")
	fmt.Fprintln(stdout)
	for s.Turn() <= 6 {
		suggestion, err := s.NextGuess()
		if err != nil {
			fail(err.Error())
		}
		detail := fmt.Sprintf("%d possible answers", len(s.Candidates()))
		if m != nil {
			guess, how, err := m.suggest(s.History, opts.hard)
			if err != nil {
				fmt.Fprintf(stdout, "  %s couldn't suggest a guess (%v); showing the solver's suggestion instead.\n", m.model, err)
				detail = "solver; " + detail
			} else {
				suggestion, detail = guess, how
			}
		}
		fmt.Fprintf(stdout, "Guess %d: %s   (%s)\n", s.Turn(), strings.ToUpper(suggestion), detail)
		var guess, pattern string
		for {
			fmt.Fprint(stdout, "  result> ")
			if !in.Scan() {
				return
			}
			guess, pattern, err = parseResult(in.Text(), suggestion)
			if err == nil {
				break
			}
			fmt.Fprintln(stdout, "  "+err.Error())
		}
		s.Record(guess, pattern)
		if pattern == wordle.Solved {
			break
		}
	}
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, shareText("", s.History))
	if opts.copy {
		copyText(shareText("", s.History))
	}
}

// parseResult reads "pattern" for the suggested word or "word pattern" for
// a word the player chose themselves.
func parseResult(line, suggestion string) (guess, pattern string, err error) {
	fields := strings.Fields(strings.ToLower(line))
	guess = suggestion
	if len(fields) > 1 && len(fields[0]) == 5 && strings.Trim(fields[0], "abcdefghijklmnopqrstuvwxyz") == "" {
		if _, perr := wordle.ParsePattern(fields[0]); perr != nil {
			guess, fields = fields[0], fields[1:]
		}
	}
	pattern, err = wordle.ParsePattern(strings.Join(fields, ""))
	return guess, pattern, err
}
