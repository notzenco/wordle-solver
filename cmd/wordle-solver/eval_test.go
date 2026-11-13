package main

import (
	modeleval "github.com/notzenco/wordle-solver/internal/eval"
	"net/http"
	"net/http/httptest"
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
