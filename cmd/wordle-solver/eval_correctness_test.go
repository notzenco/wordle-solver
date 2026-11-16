package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	modeleval "github.com/notzenco/wordle-solver/internal/eval"
)

func TestSystemOneMalformedRepliesKeepModelAndTokens(t *testing.T) {
	for name, body := range map[string]string{
		"unoffered choice":      `{"model":"routed","answers":{"guess":{"choice":"zzzzz","probabilities":{"raise":0.5,"cigar":0.5}}},"usage":{"input_tokens":123}}`,
		"invalid probabilities": `{"model":"routed","answers":{"guess":{"choice":"raise","probabilities":{"raise":2,"cigar":0}}},"usage":{"input_tokens":123}}`,
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		p := systemonePlayer{provider{s.URL, "", "systemone"}, "clef", time.Second}
		r, e := p.Guess(modeleval.Observation{Options: []string{"raise", "cigar"}})
		s.Close()
		if !errors.Is(e, modeleval.ErrMalformed) || r.Model != "routed" || r.Tokens != 123 {
			t.Errorf("%s: reply %+v, error %v", name, r, e)
		}
	}
}

func TestEvalStopsAndFailsWhenTheProviderIsDown(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(503)
	}))
	defer s.Close()
	code, out, errOut := runWith(t, "", "eval", "--home", t.TempDir(), "--base-url", s.URL, "--model", "m", "--kind", "chat", "--games", "5")
	if code == 0 || !strings.Contains(errOut, "request") {
		t.Fatalf("an outage must not look like a finished run: code %d\nstdout %s\nstderr %s", code, out, errOut)
	}
	if !strings.Contains(out, "Request failures") {
		t.Fatalf("summary hides request failures:\n%s", out)
	}
	if calls != 6 {
		t.Fatalf("want the run to stop after one fully failed game (6 requests), got %d", calls)
	}
}

func TestEvalRunRecordKeepsThePrompt(t *testing.T) {
	home := t.TempDir()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"model":"m","choices":[{"message":{"content":"raise"}}]}`))
	}))
	defer s.Close()
	out := filepath.Join(home, "run.jsonl")
	if code, _, errOut := runWith(t, "", "eval", "--home", home, "--base-url", s.URL, "--model", "m", "--kind", "chat", "--games", "1", "--out", out); code != 0 {
		t.Fatal(errOut)
	}
	b, _ := os.ReadFile(out)
	var run struct {
		Prompt        map[string]string
		OptionSeeding string `json:"option_seeding"`
	}
	json.Unmarshal([]byte(strings.SplitN(string(b), "\n", 2)[0]), &run)
	if !strings.Contains(run.Prompt["state"], "Play Wordle") || !strings.Contains(run.Prompt["instructions"], "JSON object") {
		t.Fatalf("run record lacks the prompt: %+v", run.Prompt)
	}
	if run.OptionSeeding != "per-game" {
		t.Fatalf("run record lacks option_seeding: %q", run.OptionSeeding)
	}
}

func TestModelShareCountsChargedFailures(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	answer := practiceAnswer(t, "--seed", "5")
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(503)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"model": "chat", "choices": []map[string]any{{"message": map[string]string{"content": answer}}}})
	}))
	defer s.Close()
	if err := saveConfig(configFile{Providers: map[string]provider{"local": {BaseURL: s.URL, Kind: "chat"}}}); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runWith(t, "", "auto", "--by", "model", "--model", "local/chat", "--random", "--seed", "5")
	if code != 0 || !strings.Contains(out, "2/6") {
		t.Fatalf("a failed request used a turn, so the score is 2/6: code %d\n%s\n%s", code, out, errOut)
	}
}
