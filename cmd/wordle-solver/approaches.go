package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	modeleval "github.com/notzenco/wordle-solver/internal/eval"
	"github.com/notzenco/wordle-solver/internal/player"
	"github.com/notzenco/wordle-solver/internal/wordle"
)

// An advisor suggests the next guess from the clues so far.
type advisor interface {
	// suggest returns a guess and a short note on how it was chosen.
	suggest(history []wordle.Turn, hard bool) (string, string, error)
}

// humanAdvisor is the human-like player model.
type humanAdvisor struct{ s *wordle.Solver }

func (h *humanAdvisor) suggest(history []wordle.Turn, _ bool) (string, string, error) {
	h.s.History = append([]wordle.Turn(nil), history...)
	g, err := h.s.NextGuess()
	return g, fmt.Sprintf("%d possible answers", len(h.s.Candidates())), err
}

// solverAdvisor is the entropy strategy, the best in the benchmark.
type solverAdvisor struct{ st wordle.Strategy }

func newSolverAdvisor() *solverAdvisor {
	st, _ := wordle.StrategyByName("entropy")
	return &solverAdvisor{st}
}

func (a *solverAdvisor) suggest(history []wordle.Turn, hard bool) (string, string, error) {
	g, ok := a.st.Suggest(history, hard)
	if !ok {
		return "", "", errors.New("no word fits every clue")
	}
	left := len(wordle.Filter(wordle.Answers, history))
	return g, fmt.Sprintf("entropy; %d listed answers fit", left), nil
}

// modelSuggester asks a chat or decision model for each guess, through the
// same adapters as eval. Decision models choose from up to maxOptions words
// that fit the clues so far.
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

func (m *modelSuggester) player() modeleval.Player {
	if m.kind == "systemone" {
		return &systemonePlayer{m.provider, m.model, m.timeout}
	}
	return &chatPlayer{m.provider, m.model, 0, 64, m.timeout}
}

func (m *modelSuggester) detected() {
	fmt.Fprintf(stderr, "%s is a decision model; it will choose from the words that fit the clues.\n", m.model)
	m.kind, m.autoKind = "systemone", false
}

func (m *modelSuggester) suggest(history []wordle.Turn, hard bool) (string, string, error) {
	for {
		var offered []string
		if m.kind == "systemone" {
			offered = modeleval.Options(history, m.maxOptions, 0, m.rng.Uint64())
		}
		reply, err := m.player().Guess(modeleval.Observation{History: history, Attempts: len(history), Hard: hard, Options: offered})
		if errors.Is(err, modeleval.ErrDecisionModel) && m.autoKind {
			m.detected()
			continue
		}
		if err != nil {
			return "", "", err
		}
		guess, err := modeleval.Parse(reply.Text)
		switch {
		case err != nil:
			return "", "", fmt.Errorf("unreadable reply %q", redact(m.provider, reply.Text))
		case !m.valid[guess]:
			return "", "", fmt.Errorf("%s is not a valid word", strings.ToUpper(guess))
		case hard && !wordle.HardValid(guess, history):
			return "", "", fmt.Errorf("%s breaks Hard Mode", strings.ToUpper(guess))
		}
		switch len(offered) {
		case 0:
			return guess, m.model, nil
		case 1:
			return guess, m.model + ", the only word that fits", nil
		}
		return guess, fmt.Sprintf("%s, %s of %d options", m.model, topChoices(reply.Probabilities, 3), len(offered)), nil
	}
}

// play runs one whole game through the eval harness, so the model sees only
// the colours its guesses earn. As in the game, a rejected word doesn't use a
// turn, up to three times per turn; after that it does.
func (m *modelSuggester) play(answer string, hard bool) modeleval.Result {
	for {
		r := modeleval.PlayWithOptions(m.player(), answer, m.model, hard,
			modeleval.PlayOptions{Candidates: m.kind == "systemone", MaxOptions: m.maxOptions, Seed: m.rng.Uint64(), Retries: 3})
		if r.Error == modeleval.ErrDecisionModel.Error() && m.autoKind {
			m.detected()
			continue
		}
		return r
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

// approach reads --by and --model.
func (c *cmdCtx) approach() (string, *modelSuggester, error) {
	by := c.get("by")
	switch by {
	case "human", "solver":
		if c.set("model") {
			return "", nil, usageError("--model needs --by model")
		}
		return by, nil, nil
	case "model":
		spec := c.get("model")
		if spec == "" {
			return "", nil, usageError("--by model needs --model PROVIDER/MODEL, or a saved model setting")
		}
		p, _, model, err := resolveModel(c.config, spec, "", "")
		if err != nil {
			return "", nil, err
		}
		seed := c.seed
		return by, newModelSuggester(p, model, rand.New(rand.NewPCG(seed, seed+1))), nil
	}
	return "", nil, usageError("--by must be human, solver or model")
}

// puzzle is the game being played: a published puzzle, or a random answer
// for practice.
type puzzle struct {
	number int // -1 for practice
	date   string
	answer string
}

func (p puzzle) title() string {
	if p.number < 0 {
		return "Practice puzzle"
	}
	return fmt.Sprintf("Wordle #%s for %s", thousands(p.number), p.date)
}

func (c *cmdCtx) puzzle(rng *rand.Rand) (puzzle, error) {
	date, id := c.get("date"), c.int("id")
	if c.bool("random") {
		if date != "" || id >= 0 {
			return puzzle{}, usageError("--random can't be combined with --date or --id")
		}
		return puzzle{-1, "", wordle.Answers[rng.IntN(len(wordle.Answers))]}, nil
	}
	if date != "" && id >= 0 {
		return puzzle{}, usageError("use --date or --id, not both")
	}
	day := time.Now()
	if id >= 0 {
		day = wordle.DateForNumber(id)
	}
	if date != "" {
		d, err := time.ParseInLocation(time.DateOnly, date, time.Local)
		if err != nil {
			return puzzle{}, usageError("--date must look like 2026-11-13")
		}
		day = d
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	p, err := wordle.FetchPuzzle(ctx, wordle.NYTBaseURL, day)
	if err != nil {
		return puzzle{}, err
	}
	return puzzle{p.Number, p.Date, p.Solution}, nil
}

// autoClues solves a puzzle by itself from the colours alone, with the
// entropy strategy or a model. These games are not saved.
func autoClues(by string, m *modelSuggester, p puzzle, opts options) {
	fmt.Fprintln(stdout, p.title())
	var turns []wordle.Turn
	if by == "solver" {
		fmt.Fprintln(stdout, "The entropy strategy plays from the colours alone; the answer is used only to score each guess.")
		fmt.Fprintln(stdout)
		a := newSolverAdvisor()
		for len(turns) < 6 {
			g, _, err := a.suggest(turns, opts.hard)
			if err != nil {
				fail(err.Error())
			}
			turns = append(turns, wordle.Turn{Guess: g, Pattern: wordle.Score(g, p.answer)})
			fmt.Fprintln(stdout, tiles(g, turns[len(turns)-1].Pattern))
			if turns[len(turns)-1].Pattern == wordle.Solved {
				break
			}
		}
	} else {
		fmt.Fprintf(stdout, "%s plays from the colours alone through the eval harness; it never sees the answer.\n\n", m.model)
		r := m.play(p.answer, opts.hard)
		for _, a := range r.Attempts {
			a.Reply.Text = redact(m.provider, a.Reply.Text)
			if a.Pattern != "" {
				turns = append(turns, wordle.Turn{Guess: a.Guess, Pattern: a.Pattern})
				fmt.Fprintln(stdout, tiles(a.Guess, a.Pattern))
				continue
			}
			if a.Free {
				fmt.Fprintf(stdout, "rejected, no turn used: %s %s\n", a.Problem, quoteReply(a.Reply.Text))
			} else {
				fmt.Fprintf(stdout, "turn used without a guess: %s %s\n", a.Problem, quoteReply(a.Reply.Text))
			}
		}
		if r.Error != "" {
			fail(r.Error)
		}
		if !r.Solved {
			fmt.Fprintf(stdout, "\nNot solved; the answer was %s.\n", strings.ToUpper(p.answer))
		}
	}
	fmt.Fprintln(stdout)
	number := ""
	if p.number >= 0 {
		number = thousands(p.number) + " "
	}
	fmt.Fprint(stdout, shareText(number, turns))
	if opts.copy {
		copyText(shareText(number, turns))
	}
	fmt.Fprintln(stdout, "\nNot saved: only the human-like player's games go into your history.")
}

func quoteReply(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return fmt.Sprintf("(reply %q)", s)
}

// ownGames loads the games played by hand.
func ownGames() (string, map[int]player.Game, error) {
	dir, err := player.Dir()
	if err != nil {
		return "", nil, err
	}
	games, err := player.LoadMine(dir)
	return dir, games, err
}
