package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/notzenco/wordle-solver/internal/player"
	"github.com/notzenco/wordle-solver/internal/wordle"
)

// decisionServer answers SystemOne requests with the most familiar-looking
// choice (the alphabetically first option) and rejects chat requests as
// oMLX does for decision models.
func decisionServer(t *testing.T) *httptest.Server {
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
			json.NewEncoder(w).Encode(map[string]any{"model": "clef", "answers": map[string]any{"guess": map[string]any{"choice": words[0], "confidence": probs[words[0]], "probabilities": probs}}})
		default:
			t.Error(r.URL.Path)
		}
	}))
}

func runWith(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	stdin = strings.NewReader(input)
	defer func() { stdin = nil }()
	var out, errOut bytes.Buffer
	code := runCLI(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// practiceAnswer is the answer a seeded --random game uses.
func practiceAnswer(t *testing.T, args ...string) string {
	t.Helper()
	code, out, errOut := runWith(t, "", append([]string{"auto", "--by", "solver", "--random"}, args...)...)
	if code != 0 {
		t.Fatalf("auto: %d %s", code, errOut)
	}
	m := regexp.MustCompile(`(?m)^([A-Z]) ([A-Z]) ([A-Z]) ([A-Z]) ([A-Z])   (?:🟩){5}$`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no solved row in:\n%s", out)
	}
	return strings.ToLower(strings.Join(m[1:], ""))
}

func TestPlayYourselfScoresGuessesWithTiles(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	code, out, errOut := runWith(t, "ab\nqzxvw\naahed\nhello\n", "play", "--random")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	// Like the game, rejected words don't use a guess.
	if n := strings.Count(out, "guess 1> "); n != 3 || !strings.Contains(out, "guess 2> ") || strings.Contains(out, "guess 4> ") {
		t.Errorf("rejected words used guesses:\n%s", out)
	}
	for _, want := range []string{"Practice puzzle", "type a five-letter word", "isn't in the word list", "A A H E D   ", "H E L L O   "} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "try ") {
		t.Error("play gave a hint")
	}
}

func TestAutoBySolverUsesOnlyColours(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	answer := practiceAnswer(t, "--seed", "7")
	if !wordle.InAnswers(answer) {
		t.Fatalf("practice answer %q not from the list", answer)
	}
	_, out, _ := runWith(t, "", "auto", "--by", "solver", "--random", "--seed", "7")
	if !strings.Contains(out, "from the colours alone") || !strings.Contains(out, "Not saved") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestAutoByModelRunsThroughTheHarness(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	s := decisionServer(t)
	defer s.Close()
	if err := saveConfig(configFile{Providers: map[string]provider{"local": {BaseURL: s.URL}}}); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runWith(t, "", "auto", "--by", "model", "--model", "local/clef", "--random", "--seed", "3")
	if code != 0 || !strings.Contains(out, "never sees the answer") || !strings.Contains(errOut, "decision model") {
		t.Fatalf("code %d\nstdout %s\nstderr %s", code, out, errOut)
	}
	code, _, errOut = runWith(t, "", "auto", "--by", "model", "--random")
	if code != 2 || !strings.Contains(errOut, "--model") {
		t.Fatalf("missing model: code %d, %s", code, errOut)
	}
}

func TestAssistSuggestsAfterYourFirstWord(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	s := decisionServer(t)
	defer s.Close()
	if err := saveConfig(configFile{Providers: map[string]provider{"local": {BaseURL: s.URL}}}); err != nil {
		t.Fatal(err)
	}
	answer := practiceAnswer(t, "--seed", "11")
	for _, by := range [][]string{{"--by", "human"}, {"--by", "solver"}, {"--by", "model", "--model", "local/clef"}} {
		// Take every suggestion after a first word of your own.
		args := append([]string{"assist", "--random", "--seed", "11"}, by...)
		code, out, errOut := runWith(t, "aahed\n\n\n\n\n\n", args...)
		if code != 0 {
			t.Fatalf("%v: code %d: %s", by, code, errOut)
		}
		if !strings.Contains(out, "A A H E D   ") || !strings.Contains(out, "try ") {
			t.Errorf("%v: output:\n%s", by, out)
		}
		if by[1] != "model" && !strings.Contains(out, "Solved in") {
			t.Errorf("%v did not solve %s:\n%s", by, answer, out)
		}
	}
}

func TestAssistManualTakesTypedColours(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	code, out, errOut := runWith(t, "crane\nbbbbz\nbbbbb\n\nggggg\n", "assist", "--manual", "--by", "solver")
	if code != 0 || !strings.Contains(out, "C R A N E   ⬛⬛⬛⬛⬛") || !strings.Contains(out, "Solved in 2") {
		t.Fatalf("code %d\n%s\n%s", code, out, errOut)
	}
}

func TestStatsKeepYourGamesApart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("WORDLE_SOLVER_HOME", home)
	if err := player.SaveMine(home, map[int]player.Game{1: {Date: "2021-06-20", Turns: []wordle.Turn{{Guess: "crane", Pattern: wordle.Solved}}}}); err != nil {
		t.Fatal(err)
	}
	_, mine, _ := runWith(t, "", "stats", "--mine")
	_, model, _ := runWith(t, "", "stats")
	if !strings.Contains(mine, "Played 1") || strings.Contains(model, "Played 1") {
		t.Fatalf("mine:\n%s\nmodel:\n%s", mine, model)
	}
}
