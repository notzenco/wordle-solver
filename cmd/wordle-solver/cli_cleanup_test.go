package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A flag the command would silently ignore must be rejected, so help never
// implies an option works where it doesn't.
func TestIgnoredFlagsAreRejected(t *testing.T) {
	for _, args := range [][]string{
		{"bench", "--strategy", "entropy", "--n", "10"},
		{"bench", "--strategy", "entropy", "--seed", "3"},
		{"bench", "--strategy", "entropy", "--max", "4"},
		{"bench", "--strategy", "entropy", "--target", "3"},
		{"bench", "--strategy", "entropy", "--sloppy", "0.5"},
		{"bench", "--copy"},
		{"auto", "--by", "solver", "--random", "--sloppy", "0.5"},
		{"auto", "--by", "solver", "--random", "--target", "3"},
		{"auto", "--by", "solver", "--random", "--max", "4"},
		{"assist", "--by", "solver", "--manual", "--sloppy", "0.5"},
	} {
		code, _, errOut := run(t, args...)
		if code != 2 || errOut == "" {
			t.Errorf("%v: code %d, stderr %q", args, code, errOut)
		}
	}
	if code, _, errOut := run(t, "bench", "--strategy", "freq", "--hard"); code != 0 {
		t.Errorf("--hard applies to a strategy: code %d, %q", code, errOut)
	}
}

func TestCleanHelpNamesEverythingItDeletes(t *testing.T) {
	_, out, _ := run(t, "help")
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "clean") && (!strings.Contains(line, "settings") || !strings.Contains(line, "providers")) {
			t.Fatalf("clean's summary leaves out settings or providers: %q", line)
		}
	}
}

func TestAssistManualCatchesImpossibleColoursAndCanGoBack(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	// No word can score ggggy against CRANE: a lone yellow needs a free spot.
	code, out, errOut := runWith(t, "\ncrane\nggggy\nbbbbb\nback\nslate\nggggg\n", "assist", "--manual", "--by", "solver")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	for _, want := range []string{"suggestions start after", "No word fits", "Back to word 1", "Solved in 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestProviderErrorsSayWhatWentWrong(t *testing.T) {
	t.Setenv("CHECK_TEST_KEY", "secret-value-456")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"message":"invalid api key ` + strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") + `"}}`))
	}))
	defer s.Close()
	_, err := apiRequest(&http.Client{Timeout: time.Second}, provider{BaseURL: s.URL, KeyEnv: "CHECK_TEST_KEY"}, "/models", nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") || !strings.Contains(err.Error(), "invalid api key") || strings.Contains(err.Error(), "secret-value-456") {
		t.Fatalf("status error: %v", err)
	}
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	_, err = apiRequest(&http.Client{Timeout: time.Second}, provider{BaseURL: closed.URL}, "/models", nil)
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("transport error: %v", err)
	}
}

func TestGuessesNeedsAStrategy(t *testing.T) {
	if code, _, errOut := run(t, "bench", "--guesses", "all"); code != 2 || !strings.Contains(errOut, "--strategy") {
		t.Fatalf("code %d, %q", code, errOut)
	}
	if code, _, _ := run(t, "bench", "--strategy", "freq", "--guesses", "some"); code != 2 {
		t.Fatalf("bad pool: code %d", code)
	}
}
