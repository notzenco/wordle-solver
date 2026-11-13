package main

import (
	"bytes"
	"encoding/json"
	modeleval "github.com/notzenco/wordle-solver/internal/eval"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestChatAdapter(t *testing.T) {
	t.Setenv("TEST_API_KEY", "private-test-secret")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer private-test-secret" {
			t.Error("request")
		}
		w.Write([]byte(`{"model":"routed","choices":[{"message":{"content":"raise"}}],"usage":{"total_tokens":12}}`))
	}))
	defer s.Close()
	p := chatPlayer{provider{s.URL, "TEST_API_KEY", ""}, "org/model", 0, 64, time.Second}
	r, e := p.Guess(modeleval.Observation{})
	if e != nil || r.Model != "routed" || r.Tokens != 12 {
		t.Fatal(r, e)
	}
	_, e = findProvider(configFile{}, "unknown")
	if e == nil || !strings.Contains(e.Error(), "providers add") {
		t.Fatal(e)
	}
}

func TestEvalDetectsDecisionModelAndUsesDataFolder(t *testing.T) {
	home := t.TempDir()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chat/completions":
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"Model 'clef' is not an LLM / chat model. Use /v1/systemone for decision models."}`))
		case "/systemone":
			var req struct {
				Questions struct {
					Guess struct{ Criteria map[string]string }
				}
			}
			json.NewDecoder(r.Body).Decode(&req)
			probs, first := map[string]float64{}, ""
			for w := range req.Questions.Guess.Criteria {
				probs[w] = 1 / float64(len(req.Questions.Guess.Criteria))
				if first == "" || w < first {
					first = w
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"model": "clef", "answers": map[string]any{"guess": map[string]any{"choice": first, "confidence": probs[first], "probabilities": probs}}})
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer s.Close()
	var out, errOut bytes.Buffer
	code := runCLI([]string{"eval", "--home", home, "--base-url", s.URL, "--model", "clef", "--games", "1", "--seed", "3"}, &out, &errOut)
	if code != 0 || !strings.Contains(errOut.String(), "decision model") || !strings.Contains(out.String(), "systemone, candidates") {
		t.Fatalf("code %d\nstdout %s\nstderr %s", code, out.String(), errOut.String())
	}
	files, _ := filepath.Glob(filepath.Join(home, "evals", "one-off-clef-*.jsonl"))
	if len(files) != 1 {
		t.Fatalf("want one transcript in the data folder, got %v", files)
	}
	b, _ := os.ReadFile(files[0])
	if !strings.Contains(string(b), `"kind_detected":true`) {
		t.Fatalf("run record lacks kind_detected: %s", b[:200])
	}
}

func TestEvalKeepsAnExplicitChatKind(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`not an LLM / chat model`))
	}))
	defer s.Close()
	var errOut bytes.Buffer
	code := runCLI([]string{"eval", "--home", t.TempDir(), "--base-url", s.URL, "--model", "clef", "--kind", "chat", "--games", "1"}, io.Discard, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "not a chat model") {
		t.Fatalf("code %d, stderr %s", code, errOut.String())
	}
}

func TestBuiltInAndNewProvidersDetectTheirKind(t *testing.T) {
	for name, p := range builtins {
		if p.Kind != "" {
			t.Errorf("built-in %s fixes kind %q; it should be detected", name, p.Kind)
		}
	}
	if d := optionByID("provider.kind").def; d != "" {
		t.Errorf("providers add defaults to kind %q; it should be detected", d)
	}
}
