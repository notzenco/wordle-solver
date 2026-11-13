package wordle

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchPuzzle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/svc/wordle/v2/2026-11-13.json" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"id":4745,"solution":"moxie","print_date":"2026-11-13","days_since_launch":1973}`))
	}))
	defer srv.Close()

	day := time.Date(2026, 11, 13, 0, 0, 0, 0, time.Local)
	p, err := FetchPuzzle(context.Background(), srv.URL, day)
	if err != nil {
		t.Fatal(err)
	}
	if p.Solution != "moxie" || p.Number != 1973 || p.Date != "2026-11-13" {
		t.Errorf("got %+v", p)
	}

	if _, err := FetchPuzzle(context.Background(), srv.URL, day.AddDate(0, 0, 1)); err == nil {
		t.Error("missing day gave no error")
	}
}

func TestPlaysAnswersOutsideTheList(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 5))
	for range 20 {
		h := Play("moxie", rng, 0, 0)
		if h[len(h)-1].Pattern != Solved {
			t.Fatalf("did not solve moxie: %v", h)
		}
	}
}

func TestDateForNumber(t *testing.T) {
	cases := map[int]string{0: "2021-06-19", 196: "2022-01-01", 1973: "2026-11-13"}
	for n, want := range cases {
		if got := DateForNumber(n).Format(time.DateOnly); got != want {
			t.Errorf("DateForNumber(%d) = %s, want %s", n, got, want)
		}
		if got := NumberForDate(DateForNumber(n)); got != n {
			t.Errorf("NumberForDate(%s) = %d, want %d", want, got, n)
		}
	}
}

func TestFetchPuzzleRejectsMalformedSolutions(t *testing.T) {
	for _, solution := range []string{"12!34", "MOXIE ", "mox", "moxié"} {
		body, _ := json.Marshal(map[string]any{"solution": solution, "print_date": "2026-11-13", "days_since_launch": 1973})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
		day := time.Date(2026, 11, 13, 0, 0, 0, 0, time.Local)
		if p, err := FetchPuzzle(context.Background(), srv.URL, day); err == nil {
			t.Errorf("solution %q accepted as %q", solution, p.Solution)
		}
		srv.Close()
	}
}

func TestIsWord(t *testing.T) {
	for w, want := range map[string]bool{"crane": true, "CRANE": false, "12!34": false, "cran": false, "craneс": false, "cranê": false} {
		if IsWord(w) != want {
			t.Errorf("IsWord(%q) = %v", w, !want)
		}
	}
}

func FuzzScore(f *testing.F) {
	f.Add("crane", "moxie")
	f.Add("eerie", "geese")
	f.Fuzz(func(t *testing.T, guess, answer string) {
		if !IsWord(guess) || !IsWord(answer) {
			return
		}
		p := Score(guess, answer)
		if len(p) != 5 || strings.Trim(p, "gyb") != "" || (p == Solved) != (guess == answer) {
			t.Fatalf("Score(%q, %q) = %q", guess, answer, p)
		}
	})
}
