package main

import (
	"bufio"
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

const chatInstructions = "Return only a JSON object with one key, guess, containing your next word. Do not explain."

type chatPlayer struct {
	provider    provider
	model       string
	temperature float64
	maxTokens   int
	timeout     time.Duration
}

func (p *chatPlayer) Guess(o modeleval.Observation) (modeleval.Reply, error) {
	prompt := gameState(o) + chatInstructions
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
	jobs                              int
	resume                            bool // r.out is an interrupted transcript to finish
}

// errDetected reports that the first request reached a decision model while
// the chat adapter was in use, before anything was scored.
var errDetected = errors.New("decision model detected")

func evalCommand(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	jobs := c.int("eval.jobs")
	if jobs < 1 {
		return usageError("--jobs must be at least 1")
	}
	if path := c.get("eval.resume"); path != "" {
		// The transcript fixes everything that decides the games.
		if err := c.unused("--resume takes the run's settings from its transcript", "eval.model", "eval.games", "eval.set", "eval.seed",
			"hard", "eval.kind", "eval.options", "eval.max-options", "eval.distractors", "eval.temperature", "eval.max-tokens",
			"eval.timeout", "eval.out"); err != nil {
			return err
		}
		r, err := resumedRun(c, path)
		if err != nil {
			return err
		}
		r.jobs = jobs
		return runEval(r)
	}
	spec := c.get("eval.model")
	if spec == "" {
		return usageError("eval needs --model PROVIDER/MODEL, or a saved eval.model setting")
	}
	r := evalRun{set: c.get("eval.set"), games: c.int("eval.games"), seed: c.uint("eval.seed"), hard: c.bool("hard"),
		maxOptions: c.int("eval.max-options"), distractors: c.int("eval.distractors"), temperature: c.float("eval.temperature"),
		maxTokens: c.int("eval.max-tokens"), timeout: c.dur("eval.timeout"), out: c.get("eval.out"), jobs: jobs}
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
	s := evalSummary{Model: r.model, Kind: r.kind, Options: r.mode, Out: r.out, Games: len(answers), Distractors: r.distractors}
	var f *os.File
	var err error
	done := 0
	if r.resume {
		var played []modeleval.Result
		if played, err = readTranscript(r.out); err != nil {
			return err
		}
		for _, g := range played {
			s.record(g)
		}
		done = len(played)
		// Appending keeps what was played; nothing earlier is rewritten.
		if f, err = os.OpenFile(r.out, os.O_APPEND|os.O_WRONLY, 0600); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(r.out), 0700); err != nil {
			return err
		}
		// Exclusive creation protects previous transcripts.
		if f, err = os.OpenFile(r.out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600); err != nil {
			return err
		}
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	if !r.resume {
		if err = enc.Encode(runRecord(r)); err != nil {
			return err
		}
	}
	var player modeleval.Player = &chatPlayer{r.provider, r.model, r.temperature, r.maxTokens, r.timeout}
	if r.kind == "systemone" {
		player = &systemonePlayer{r.provider, r.model, r.timeout}
	}
	// Progress goes to stderr so stdout keeps only the summary. Each game is
	// written to the transcript as it ends, so an interrupted run keeps them.
	if done > 0 {
		fmt.Fprintf(stderr, "%s: resuming at game %d/%d; transcript %s\n", r.model, done+1, len(answers), r.out)
	} else {
		fmt.Fprintf(stderr, "%s: %d games, up to %d requests; transcript %s\n", r.model, len(answers), 6*(len(answers)-done), r.out)
	}
	play := func(i int) modeleval.Result {
		return modeleval.PlayWithOptions(player, answers[i], r.model, r.hard, modeleval.PlayOptions{Candidates: r.mode == "candidates", MaxOptions: r.maxOptions, Distractors: r.distractors, Seed: r.seed, Game: i})
	}
	// The first game runs alone: it may reveal a decision model, and then
	// nothing else should be sent.
	stop := make(chan struct{})
	defer close(stop)
	results := playInOrder(done, len(answers), max(1, r.jobs), play, stop)
	for i := done; i < len(answers); i++ {
		g := <-results
		if i == 0 && r.autoKind && g.Error == modeleval.ErrDecisionModel.Error() {
			f.Close()
			os.Remove(r.out)
			return errDetected
		}
		if g.RequestFailures == g.Guesses {
			// A whole game of failed requests is an outage, not a model
			// result, so it is left out and --resume plays it again.
			s.Games = i
			writeEvalSummary(stdout, s)
			return fmt.Errorf("every request in game %d failed (%s); stopped without recording it, so the outage is not scored as model play (transcript so far: %s; --resume continues it)",
				i+1, redact(r.provider, g.Attempts[len(g.Attempts)-1].Error), r.out)
		}
		s.record(g)
		fmt.Fprintf(stderr, "game %d/%d: %s\n", i+1, len(answers), gameProgress(g))
		// Redact an echoed configured key before persisting provider-controlled strings.
		for j := range g.Attempts {
			g.Attempts[j].Reply.Text = redact(r.provider, g.Attempts[j].Reply.Text)
			g.Attempts[j].Reply.Model = redact(r.provider, g.Attempts[j].Reply.Model)
			g.Attempts[j].Error = redact(r.provider, g.Attempts[j].Error)
		}
		if err = enc.Encode(map[string]any{"type": "game", "game": g}); err != nil {
			return err
		}
		if g.Error != "" {
			return fmt.Errorf("%s (transcript so far: %s)", g.Error, r.out)
		}
	}
	if err = enc.Encode(map[string]any{"type": "summary", "metrics": s.metrics(), "games": s.Games, "eligible": s.Eligible, "solved": s.Solved, "rerouted": s.Rerouted, "request_failures": s.RequestFailures}); err != nil {
		return err
	}
	writeEvalSummary(stdout, s)
	return f.Sync()
}

// playInOrder plays games from..n-1 with up to jobs at once and delivers
// the results in game order. The first game plays alone. Once stop is
// closed nothing more starts; games already running finish unread.
func playInOrder(from, n, jobs int, play func(int) modeleval.Result, stop <-chan struct{}) <-chan modeleval.Result {
	out := make(chan modeleval.Result)
	go func() {
		defer close(out)
		send := func(g modeleval.Result) bool {
			select {
			case out <- g:
				return true
			case <-stop:
				return false
			}
		}
		if from >= n || !send(play(from)) {
			return
		}
		pending := map[int]chan modeleval.Result{}
		next := from + 1
		for i := from + 1; i < n; i++ {
			// Keep jobs games running, starting the next as each result goes
			// out, so the transcript stays in order without idle workers.
			for len(pending) < jobs && next < n {
				ch := make(chan modeleval.Result, 1)
				pending[next] = ch
				go func(k int) { ch <- play(k) }(next)
				next++
			}
			g := <-pending[i]
			delete(pending, i)
			if !send(g) {
				return
			}
		}
	}()
	return out
}

func runRecord(r evalRun) map[string]any {
	settings := map[string]any{"eval.set": r.set, "eval.games": r.games, "eval.seed": r.seed, "hard": r.hard, "eval.kind": r.kind,
		"eval.options": r.mode, "eval.max-options": r.maxOptions, "eval.distractors": r.distractors,
		"eval.temperature": r.temperature, "eval.max-tokens": r.maxTokens, "eval.timeout": r.timeout.String()}
	// The prompt is recorded so a later wording change can't silently make
	// runs incomparable.
	prompt := map[string]string{"state": gameState(modeleval.Observation{Hard: r.hard}), "instructions": chatInstructions}
	if r.kind == "systemone" {
		prompt["instructions"] = systemoneInstructions
	}
	return map[string]any{"type": "run", "option_seeding": "per-game", "prompt": prompt, "model": r.model, "provider": r.name, "base_url": r.provider.BaseURL, "settings": settings,
		"kind_detected": r.detected, "seed": r.seed, "date": time.Now().UTC().Format(time.RFC3339), "version": versionString()}
}

// transcriptRun is the part of a run record that resuming needs.
type transcriptRun struct {
	Type, Model, Provider string
	BaseURL               string `json:"base_url"`
	OptionSeeding         string `json:"option_seeding"`
	Settings              struct {
		Set         string  `json:"eval.set"`
		Games       int     `json:"eval.games"`
		Seed        uint64  `json:"eval.seed"`
		Hard        bool    `json:"hard"`
		Kind        string  `json:"eval.kind"`
		Options     string  `json:"eval.options"`
		MaxOptions  int     `json:"eval.max-options"`
		Distractors int     `json:"eval.distractors"`
		Temperature float64 `json:"eval.temperature"`
		MaxTokens   int     `json:"eval.max-tokens"`
		Timeout     string  `json:"eval.timeout"`
	}
}

func scanTranscript(path string, each func(kind string, line []byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 64<<20)
	for n := 1; sc.Scan(); n++ {
		var t struct{ Type string }
		if err := json.Unmarshal(sc.Bytes(), &t); err != nil {
			return fmt.Errorf("%s line %d is not a complete record (%v); remove it to resume", path, n, err)
		}
		if err := each(t.Type, sc.Bytes()); err != nil {
			return err
		}
	}
	return sc.Err()
}

// resumedRun rebuilds a run's configuration from its transcript. Only the
// connection (--base-url, --api-key-env) and --jobs may differ.
func resumedRun(c *cmdCtx, path string) (evalRun, error) {
	var t transcriptRun
	err := scanTranscript(path, func(kind string, line []byte) error {
		switch kind {
		case "run":
			return json.Unmarshal(line, &t)
		case "summary":
			return fmt.Errorf("%s is already complete", path)
		}
		return nil
	})
	if err != nil {
		return evalRun{}, err
	}
	if t.Type != "run" {
		return evalRun{}, fmt.Errorf("%s has no run record", path)
	}
	if t.OptionSeeding != "per-game" && t.Settings.Options == "candidates" {
		return evalRun{}, fmt.Errorf("%s predates per-game option sampling, so resuming would mix two samplings", path)
	}
	timeout, err := time.ParseDuration(t.Settings.Timeout)
	if err != nil {
		return evalRun{}, fmt.Errorf("%s: bad eval.timeout %q", path, t.Settings.Timeout)
	}
	baseURL := c.get("eval.base-url")
	if baseURL == "" && t.Provider == "one-off" {
		baseURL = t.BaseURL
	}
	spec := t.Provider + "/" + t.Model
	if baseURL != "" {
		spec = t.Model
	}
	p, name, model, err := resolveModel(c.config, spec, baseURL, c.get("eval.api-key-env"))
	if err != nil {
		return evalRun{}, err
	}
	st := t.Settings
	return evalRun{name: name, model: model, kind: st.Kind, mode: st.Options, set: st.Set, out: path, provider: p,
		games: st.Games, maxOptions: st.MaxOptions, distractors: st.Distractors, maxTokens: st.MaxTokens, seed: st.Seed,
		temperature: st.Temperature, timeout: timeout, hard: st.Hard, resume: true}, nil
}

// readTranscript returns the games a transcript already holds, in order.
func readTranscript(path string) ([]modeleval.Result, error) {
	var games []modeleval.Result
	err := scanTranscript(path, func(kind string, line []byte) error {
		if kind != "game" {
			return nil
		}
		var g struct{ Game modeleval.Result }
		if err := json.Unmarshal(line, &g); err != nil {
			return err
		}
		games = append(games, g.Game)
		return nil
	})
	return games, err
}

func gameProgress(g modeleval.Result) string {
	result := fmt.Sprintf("solved in %d", g.Guesses)
	if !g.Solved {
		result = "not solved"
	}
	if g.Rerouted {
		result += ", rerouted"
	}
	if g.RequestFailures > 0 {
		result += fmt.Sprintf(", %d failed requests", g.RequestFailures)
	}
	return fmt.Sprintf("%s (entropy %d)", result, g.Baseline)
}

// record counts one game: rerouted games apart, every other one as eligible.
func (s *evalSummary) record(g modeleval.Result) {
	s.RequestFailures += g.RequestFailures
	if g.Rerouted {
		s.Rerouted++
		return
	}
	s.add(g)
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
