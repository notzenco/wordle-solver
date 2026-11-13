package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	modeleval "github.com/notzenco/wordle-solver/internal/eval"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
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
func evalCommand(fs *flag.FlagSet, c configFile) error {
	value := func(n string) string { return fs.Lookup(n).Value.String() }
	spec := value("model")
	name, model, e := modeleval.SplitModel(spec)
	var p provider
	if value("base-url") != "" {
		p = provider{value("base-url"), value("api-key-env"), ""}
		if e != nil {
			model = spec
			name = "one-off"
			if model == "" {
				return fmt.Errorf("--model is required")
			}
		}
	} else {
		if e != nil {
			return e
		}
		p, e = findProvider(c, name)
		if e != nil {
			return e
		}
		if value("api-key-env") != "" {
			p.KeyEnv = value("api-key-env")
		}
	}
	kind := providerKind(p)
	if value("kind") != "" {
		kind = value("kind")
	}
	if kind == "systemone" && value("options") != "candidates" {
		return fmt.Errorf("systemone requires --options candidates")
	}
	p.Kind = kind
	cap, _ := strconv.Atoi(value("max-options"))
	distractors, _ := strconv.Atoi(value("distractors"))
	if value("options") != "candidates" && distractors > 0 {
		return fmt.Errorf("--distractors requires --options candidates")
	}
	games, _ := strconv.Atoi(value("games"))
	seed, _ := strconv.ParseUint(value("seed"), 10, 64)
	temperature, _ := strconv.ParseFloat(value("temperature"), 64)
	maxTokens, _ := strconv.Atoi(value("max-tokens"))
	timeout, _ := time.ParseDuration(value("timeout"))
	hard, _ := strconv.ParseBool(value("hard"))
	answers := modeleval.Sample(value("set"), games, seed)
	// Exclusive creation protects previous transcripts.
	f, e := os.OpenFile(value("out"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	settings := map[string]string{}
	for k, n := range settingFlags {
		settings[k] = value(n)
	}
	settings["eval.kind"] = kind
	meta := map[string]any{"type": "run", "model": model, "provider": name, "base_url": p.BaseURL, "settings": settings, "seed": seed, "date": time.Now().UTC().Format(time.RFC3339), "version": versionString()}
	if e = enc.Encode(meta); e != nil {
		return e
	}
	var player modeleval.Player = &chatPlayer{p, model, temperature, maxTokens, timeout}
	if kind == "systemone" {
		player = &systemonePlayer{p, model, timeout}
	}
	metricCount, inconsistent, choices := 0, 0, 0
	probability, loss, uniform, frequency := 0.0, 0.0, 0.0, 0.0
	solved, eligible, rerouted := 0, 0, 0
	guesses, beyond, invalid, malformed, violations, tokens := 0, 0, 0, 0, 0, 0
	var duration time.Duration
	for _, a := range answers {
		r := modeleval.PlayWithOptions(player, a, model, hard, modeleval.PlayOptions{Candidates: value("options") == "candidates", MaxOptions: cap, Distractors: distractors, Seed: seed})
		if r.Rerouted {
			rerouted++
		} else {
			eligible++
			for _, attempt := range r.Attempts {
				if attempt.Problem == "" || attempt.Problem == "hard mode violation" {
					choices++
					if attempt.Inconsistent {
						inconsistent++
					}
				}
				if m := attempt.Metrics; m != nil {
					metricCount++
					probability += m.TrueProbability
					loss += m.LogLoss
					uniform += m.UniformLogLoss
					frequency += m.FrequencyLogLoss
				}
			}
			guesses += r.Guesses
			beyond += r.Beyond
			invalid += r.Invalid
			malformed += r.Malformed
			violations += r.HardViolations
			tokens += r.Tokens
			duration += r.Duration
			if r.Solved {
				solved++
			}
		}
		// Redact an echoed configured key before persisting provider-controlled strings.
		if key := os.Getenv(p.KeyEnv); key != "" {
			for i := range r.Attempts {
				r.Attempts[i].Reply.Text = strings.ReplaceAll(r.Attempts[i].Reply.Text, key, "[redacted]")
				r.Attempts[i].Reply.Model = strings.ReplaceAll(r.Attempts[i].Reply.Model, key, "[redacted]")
			}
		}
		if e = enc.Encode(map[string]any{"type": "game", "game": r}); e != nil {
			return e
		}
		if r.Error != "" {
			return fmt.Errorf("%s", r.Error)
		}
	}
	metrics := map[string]any{"offered_answer_turns": metricCount, "model_log_loss": 0.0, "uniform_log_loss": 0.0, "frequency_log_loss": 0.0, "true_probability": 0.0, "inconsistent_rate": 0.0}
	if metricCount > 0 {
		n := float64(metricCount)
		metrics["model_log_loss"] = loss / n
		metrics["uniform_log_loss"] = uniform / n
		metrics["frequency_log_loss"] = frequency / n
		metrics["true_probability"] = probability / n
	}
	if choices > 0 {
		metrics["inconsistent_rate"] = float64(inconsistent) / float64(choices)
	}
	if e = enc.Encode(map[string]any{"metrics": metrics, "type": "summary", "games": len(answers), "eligible": eligible, "solved": solved, "rerouted": rerouted}); e != nil {
		return e
	}
	writeEvalSummary(os.Stdout, evalSummary{
		Model: model, Kind: kind, Options: value("options"), Out: value("out"),
		Games: len(answers), Eligible: eligible, Solved: solved, Rerouted: rerouted,
		Guesses: guesses, Beyond: beyond, Invalid: invalid, Malformed: malformed, Violations: violations, Tokens: tokens, Duration: duration,
		MetricCount: metricCount, Choices: choices, Inconsistent: inconsistent, Probability: probability, Loss: loss, Uniform: uniform, Frequency: frequency, Distractors: distractors,
	})
	return f.Sync()
}
