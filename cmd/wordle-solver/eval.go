package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	modeleval "github.com/notzenco/wordle-solver/internal/eval"
	"github.com/notzenco/wordle-solver/internal/player"
)

type chatPlayer struct {
	provider    provider
	model       string
	temperature float64
	maxTokens   int
	timeout     time.Duration
}

func (p *chatPlayer) Guess(o modeleval.Observation) (modeleval.Reply, error) {
	prompt := gameState(o) + "Return only a JSON object with one key, guess, containing your next word. Do not explain."
	if o.Options != nil {
		prompt += " Choose only from: " + strings.Join(o.Options, ", ")
	}
	body, _ := json.Marshal(map[string]any{"model": p.model, "temperature": p.temperature, "max_tokens": p.maxTokens, "messages": []map[string]string{{"role": "user", "content": prompt}}})
	b, e := apiRequest(&http.Client{Timeout: p.timeout}, p.provider, "/chat/completions", bytes.NewReader(body))
	if e != nil {
		return modeleval.Reply{}, e
	}
	var r struct {
		Model   string
		Choices []struct{ Message struct{ Content string } }
		Usage   struct {
			TotalTokens int `json:"total_tokens"`
		}
	}
	if json.Unmarshal(b, &r) != nil || len(r.Choices) == 0 {
		return modeleval.Reply{}, fmt.Errorf("invalid completion response")
	}
	// Some providers echo secrets in errors; only successful content is retained.
	return modeleval.Reply{Text: r.Choices[0].Message.Content, Model: r.Model, Tokens: r.Usage.TotalTokens}, nil
}

// evalRun is one fully resolved eval configuration.
type evalRun struct {
	name, model, kind, mode, set, out string
	provider                          provider
	games, maxOptions, distractors    int
	maxTokens                         int
	seed                              uint64
	temperature                       float64
	timeout                           time.Duration
	hard, detected, autoKind          bool
}

// errDetected reports that the first request reached a decision model while
// the chat adapter was in use, before anything was scored.
var errDetected = errors.New("decision model detected")

func evalCommand(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	spec := c.get("eval.model")
	if spec == "" {
		return usageError("eval needs --model PROVIDER/MODEL, or a saved eval.model setting")
	}
	r := evalRun{set: c.get("eval.set"), games: c.int("eval.games"), seed: c.uint("eval.seed"), hard: c.bool("hard"),
		maxOptions: c.int("eval.max-options"), distractors: c.int("eval.distractors"), temperature: c.float("eval.temperature"),
		maxTokens: c.int("eval.max-tokens"), timeout: c.dur("eval.timeout"), out: c.get("eval.out")}
	var err error
	r.provider, r.name, r.model, err = resolveModel(c.config, spec, c.get("eval.base-url"), c.get("eval.api-key-env"))
	if err != nil {
		return err
	}
	name, model := r.name, r.model
	r.kind = c.get("eval.kind")
	r.autoKind = r.kind == "" && r.provider.Kind == ""
	if r.kind == "" {
		r.kind = providerKind(r.provider)
	}
	mode := c.get("eval.options")
	if r.out == "" {
		dir, err := player.Dir()
		if err != nil {
			return err
		}
		r.out = filepath.Join(dir, "evals", fmt.Sprintf("%s-%s-%s.jsonl", name, fileSafe(model), time.Now().Format("20060102-150405")))
	}
	for {
		r.mode = mode
		if r.mode == "" {
			r.mode = map[string]string{"chat": "free", "systemone": "candidates"}[r.kind]
		}
		if r.kind == "systemone" && r.mode != "candidates" {
			return usageError("decision models (--kind systemone) choose from offered words; use --options candidates")
		}
		if r.distractors > 0 && r.mode != "candidates" {
			return usageError("--distractors needs --options candidates")
		}
		err := runEval(r)
		if errors.Is(err, errDetected) && !r.detected {
			fmt.Fprintf(stderr, "%s is a decision model; using --kind systemone with candidate options.\n", model)
			r.kind, r.detected, r.autoKind = "systemone", true, false
			continue
		}
		return err
	}
}

// resolveModel finds the provider for PROVIDER/MODEL, or uses baseURL as a
// one-off server with spec as the bare model id.
func resolveModel(c configFile, spec, baseURL, keyEnv string) (provider, string, string, error) {
	name, model, err := modeleval.SplitModel(spec)
	if baseURL != "" {
		if err != nil {
			name, model = "one-off", spec
		}
		return provider{BaseURL: baseURL, KeyEnv: keyEnv}, name, model, nil
	}
	if err != nil {
		return provider{}, "", "", usageError("--model must be PROVIDER/MODEL, such as ollama/llama3.2 (or pass --base-url for a one-off server)")
	}
	p, err := findProvider(c, name)
	if err != nil {
		return p, "", "", err
	}
	if keyEnv != "" {
		p.KeyEnv = keyEnv
	}
	return p, name, model, nil
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func fileSafe(s string) string { return strings.Trim(unsafeFileChars.ReplaceAllString(s, "-"), "-") }

func runEval(r evalRun) error {
	answers := modeleval.Sample(r.set, r.games, r.seed)
	if err := os.MkdirAll(filepath.Dir(r.out), 0700); err != nil {
		return err
	}
	// Exclusive creation protects previous transcripts.
	f, err := os.OpenFile(r.out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	settings := map[string]any{"eval.set": r.set, "eval.games": r.games, "eval.seed": r.seed, "hard": r.hard, "eval.kind": r.kind,
		"eval.options": r.mode, "eval.max-options": r.maxOptions, "eval.distractors": r.distractors,
		"eval.temperature": r.temperature, "eval.max-tokens": r.maxTokens, "eval.timeout": r.timeout.String()}
	meta := map[string]any{"type": "run", "model": r.model, "provider": r.name, "base_url": r.provider.BaseURL, "settings": settings,
		"kind_detected": r.detected, "seed": r.seed, "date": time.Now().UTC().Format(time.RFC3339), "version": versionString()}
	if err = enc.Encode(meta); err != nil {
		return err
	}
	var player modeleval.Player = &chatPlayer{r.provider, r.model, r.temperature, r.maxTokens, r.timeout}
	if r.kind == "systemone" {
		player = &systemonePlayer{r.provider, r.model, r.timeout}
	}
	s := evalSummary{Model: r.model, Kind: r.kind, Options: r.mode, Out: r.out, Games: len(answers), Distractors: r.distractors}
	for i, a := range answers {
		g := modeleval.PlayWithOptions(player, a, r.model, r.hard, modeleval.PlayOptions{Candidates: r.mode == "candidates", MaxOptions: r.maxOptions, Distractors: r.distractors, Seed: r.seed})
		if i == 0 && r.autoKind && g.Error == modeleval.ErrDecisionModel.Error() {
			f.Close()
			os.Remove(r.out)
			return errDetected
		}
		if g.Rerouted {
			s.Rerouted++
		} else {
			s.add(g)
		}
		// Redact an echoed configured key before persisting provider-controlled strings.
		if key := os.Getenv(r.provider.KeyEnv); key != "" {
			for i := range g.Attempts {
				g.Attempts[i].Reply.Text = strings.ReplaceAll(g.Attempts[i].Reply.Text, key, "[redacted]")
				g.Attempts[i].Reply.Model = strings.ReplaceAll(g.Attempts[i].Reply.Model, key, "[redacted]")
			}
		}
		if err = enc.Encode(map[string]any{"type": "game", "game": g}); err != nil {
			return err
		}
		if g.Error != "" {
			return fmt.Errorf("%s (transcript so far: %s)", g.Error, r.out)
		}
	}
	if err = enc.Encode(map[string]any{"type": "summary", "metrics": s.metrics(), "games": s.Games, "eligible": s.Eligible, "solved": s.Solved, "rerouted": s.Rerouted}); err != nil {
		return err
	}
	writeEvalSummary(stdout, s)
	return f.Sync()
}

func (s *evalSummary) add(g modeleval.Result) {
	s.Eligible++
	for _, a := range g.Attempts {
		if a.Problem == "" || a.Problem == "hard mode violation" {
			s.Choices++
			if a.Inconsistent {
				s.Inconsistent++
			}
		}
		if m := a.Metrics; m != nil {
			s.MetricCount++
			s.Probability += m.TrueProbability
			s.Loss += m.LogLoss
			s.Uniform += m.UniformLogLoss
			s.Frequency += m.FrequencyLogLoss
		}
	}
	s.Guesses += g.Guesses
	s.Beyond += g.Beyond
	s.Invalid += g.Invalid
	s.Malformed += g.Malformed
	s.Violations += g.HardViolations
	s.Tokens += g.Tokens
	s.Duration += g.Duration
	if g.Solved {
		s.Solved++
	}
}

func (s evalSummary) metrics() map[string]any {
	m := map[string]any{"offered_answer_turns": s.MetricCount, "model_log_loss": 0.0, "uniform_log_loss": 0.0, "frequency_log_loss": 0.0, "true_probability": 0.0, "inconsistent_rate": 0.0}
	if s.MetricCount > 0 {
		n := float64(s.MetricCount)
		m["model_log_loss"], m["uniform_log_loss"], m["frequency_log_loss"], m["true_probability"] = s.Loss/n, s.Uniform/n, s.Frequency/n, s.Probability/n
	}
	if s.Choices > 0 {
		m["inconsistent_rate"] = float64(s.Inconsistent) / float64(s.Choices)
	}
	return m
}
