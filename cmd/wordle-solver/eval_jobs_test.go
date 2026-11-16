package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// slowChat answers every request with the first fitting word it is shown,
// after a short delay, and records the most requests in flight at once.
func slowChat(t *testing.T) (*httptest.Server, *int) {
	var mu sync.Mutex
	inFlight, most := 0, 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inFlight++
		most = max(most, inFlight)
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
		w.Write([]byte(`{"model":"m","choices":[{"message":{"content":"raise"}}]}`))
	}))
	t.Cleanup(s.Close)
	return s, &most
}

// records reads a transcript's records, dropping run-specific fields so two
// runs of the same games compare equal.
func records(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 16<<20)
	for sc.Scan() {
		var v map[string]any
		json.Unmarshal(sc.Bytes(), &v)
		switch v["type"] {
		case "run":
			continue
		case "game":
			g := v["game"].(map[string]any)
			delete(g, "Duration")
			for _, a := range g["Attempts"].([]any) {
				delete(a.(map[string]any), "Duration")
			}
		}
		b, _ := json.Marshal(v)
		out = append(out, string(b))
	}
	return out
}

func TestEvalJobsRunGamesConcurrentlyInOrder(t *testing.T) {
	s, most := slowChat(t)
	home := t.TempDir()
	one, four := filepath.Join(home, "one.jsonl"), filepath.Join(home, "four.jsonl")
	base := []string{"eval", "--home", home, "--base-url", s.URL, "--model", "m", "--kind", "chat", "--games", "8", "--seed", "4"}
	if code, _, errOut := runWith(t, "", append(base, "--out", one)...); code != 0 {
		t.Fatal(errOut)
	}
	*most = 0
	if code, _, errOut := runWith(t, "", append(base, "--out", four, "--jobs", "4")...); code != 0 {
		t.Fatal(errOut)
	}
	if *most < 2 {
		t.Errorf("--jobs 4 never had more than %d requests in flight", *most)
	}
	a, b := records(t, one), records(t, four)
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Fatalf("--jobs changed the transcript:\n%s\n---\n%s", strings.Join(a, "\n"), strings.Join(b, "\n"))
	}
	if code, _, _ := runWith(t, "", append(base, "--jobs", "0")...); code != 2 {
		t.Errorf("--jobs 0: code %d", code)
	}
}

func TestEvalResumeFinishesAnInterruptedRun(t *testing.T) {
	s, _ := slowChat(t)
	home := t.TempDir()
	full, cut := filepath.Join(home, "full.jsonl"), filepath.Join(home, "cut.jsonl")
	if code, _, errOut := runWith(t, "", "eval", "--home", home, "--base-url", s.URL, "--model", "m", "--kind", "chat", "--games", "5", "--seed", "9", "--out", full); code != 0 {
		t.Fatal(errOut)
	}
	// Keep the run record and the first two games, as after a Ctrl-C.
	b, _ := os.ReadFile(full)
	lines := strings.SplitAfter(string(b), "\n")
	os.WriteFile(cut, []byte(strings.Join(lines[:3], "")), 0o600)
	code, out, errOut := runWith(t, "", "eval", "--home", home, "--base-url", s.URL, "--resume", cut)
	if code != 0 || !strings.Contains(errOut, "resuming at game 3/5") {
		t.Fatalf("code %d\n%s\n%s", code, out, errOut)
	}
	if a, b := records(t, full), records(t, cut); strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Fatalf("resumed transcript differs:\n%s\n---\n%s", strings.Join(a, "\n"), strings.Join(b, "\n"))
	}
	if code, _, errOut := runWith(t, "", "eval", "--home", home, "--base-url", s.URL, "--resume", cut); code == 0 || !strings.Contains(errOut, "already complete") {
		t.Fatalf("resuming a finished run: code %d, %s", code, errOut)
	}
	if code, _, _ := runWith(t, "", "eval", "--home", home, "--base-url", s.URL, "--resume", cut, "--games", "9"); code != 2 {
		t.Fatalf("--resume takes its settings from the transcript; --games should be refused")
	}
}
