package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// decisionServer answers SystemOne requests with the alphabetically first
// option, or with choose when set, and rejects chat requests as oMLX does.
func decisionServer(t *testing.T, choose string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chat/completions":
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"Model 'clef' is not an LLM / chat model."}`))
		case "/systemone":
			var req struct {
				Questions struct {
					Guess struct{ Criteria map[string]string }
				}
			}
			json.NewDecoder(r.Body).Decode(&req)
			words := []string{}
			for w := range req.Questions.Guess.Criteria {
				words = append(words, w)
			}
			sort.Strings(words)
			probs := map[string]float64{}
			for _, w := range words {
				probs[w] = 1 / float64(len(words))
			}
			pick := words[0]
			if choose != "" {
				pick = choose
			}
			json.NewEncoder(w).Encode(map[string]any{"model": "clef", "answers": map[string]any{"guess": map[string]any{"choice": pick, "confidence": probs[words[0]], "probabilities": probs}}})
		default:
			t.Error(r.URL.Path)
		}
	}))
}

func playWith(t *testing.T, s *httptest.Server, input string) (int, string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("WORDLE_SOLVER_HOME", home)
	if err := saveConfig(configFile{Providers: map[string]provider{"local": {BaseURL: s.URL}}}); err != nil {
		t.Fatal(err)
	}
	stdin = strings.NewReader(input)
	defer func() { stdin = nil }()
	var out, errOut bytes.Buffer
	code := runCLI([]string{"play", "--model", "local/clef", "--seed", "1"}, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestPlayTakesSuggestionsFromAModel(t *testing.T) {
	s := decisionServer(t, "")
	defer s.Close()
	code, out, errOut := playWith(t, s, "ggggg\n")
	if code != 0 || !strings.Contains(out, "clef") || !strings.Contains(out, "options") || !strings.Contains(errOut, "decision model") {
		t.Fatalf("code %d\nstdout %s\nstderr %s", code, out, errOut)
	}
}

func TestPlayFallsBackWhenTheModelReplyIsUnusable(t *testing.T) {
	s := decisionServer(t, "zzzzz")
	defer s.Close()
	code, out, _ := playWith(t, s, "ggggg\n")
	if code != 0 || !strings.Contains(out, "solver's suggestion instead") {
		t.Fatalf("code %d\nstdout %s", code, out)
	}
}
