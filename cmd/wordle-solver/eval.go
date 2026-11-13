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
	history, _ := json.Marshal(o.History)
	prompt := fmt.Sprintf("Play Wordle. Guess a five-letter English word within six attempts. Feedback: g=green (correct position), y=yellow (elsewhere), b=grey (absent); repeated letters use remaining counts. Hard Mode=%t: retain greens and reuse all revealed letters including multiplicity. Attempts already used: %d. History: %s. Return only a JSON object with one key, guess, containing your next word. Do not explain.", o.Hard, o.Attempts, history)
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
		p = provider{value("base-url"), value("api-key-env")}
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
	meta := map[string]any{"type": "run", "model": model, "provider": name, "base_url": p.BaseURL, "settings": settings, "seed": seed, "date": time.Now().UTC().Format(time.RFC3339), "version": versionString()}
	if e = enc.Encode(meta); e != nil {
		return e
	}
	player := &chatPlayer{p, model, temperature, maxTokens, timeout}
	solved, eligible, rerouted := 0, 0, 0
	guesses, beyond, invalid, malformed, violations, tokens := 0, 0, 0, 0, 0, 0
	var duration time.Duration
	for _, a := range answers {
		r := modeleval.Play(player, a, model, hard)
		if r.Rerouted {
			rerouted++
		} else {
			eligible++
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
	}
	if e = enc.Encode(map[string]any{"type": "summary", "games": len(answers), "eligible": eligible, "solved": solved, "rerouted": rerouted}); e != nil {
		return e
	}
	fmt.Printf("model\tgames\teligible\tsolved\trerouted\n%s\t%d\t%d\t%d\t%d\n", model, len(answers), eligible, solved, rerouted)
	fmt.Printf("eligible attempts\tbeyond entropy\tinvalid\tmalformed\thard violations\ttokens\ttime\n%d\t%d\t%d\t%d\t%d\t%d\t%s\n", guesses, beyond, invalid, malformed, violations, tokens, duration)
	return f.Sync()
}
