package main

import (
	"encoding/json"
	"errors"
	modeleval "github.com/notzenco/wordle-solver/internal/eval"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSystemOne(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/systemone" {
			t.Error(r.URL.Path)
		}
		var body struct {
			State     string
			Questions map[string]struct {
				Type     string
				Criteria map[string]string
			}
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.State == "" || body.Questions["guess"].Criteria["raise"] != "RAISE" {
			t.Error(body)
		}
		w.Write([]byte(`{"model":"clef","answers":{"guess":{"choice":"raise","confidence":0.75,"probabilities":{"raise":0.75,"cigar":0.25}}},"usage":{"input_tokens":120}}`))
	}))
	defer s.Close()
	p := systemonePlayer{provider{s.URL, "", "systemone"}, "clef", time.Second}
	r, e := p.Guess(modeleval.Observation{Options: []string{"raise", "cigar"}})
	if e != nil || r.Text != "raise" || r.Tokens != 120 || r.Confidence != 0.75 || r.Probabilities["cigar"] != 0.25 {
		t.Fatal(r, e)
	}
}
func TestDecisionMismatch(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"message":"not an LLM / chat model"}}`))
	}))
	defer s.Close()
	p := chatPlayer{provider{s.URL, "", "chat"}, "clef", 0, 64, time.Second}
	_, e := p.Guess(modeleval.Observation{})
	if !errors.Is(e, modeleval.ErrDecisionModel) || calls != 1 {
		t.Fatal(e, calls)
	}
}

func TestProviderKind(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	if code := runCLI([]string{"providers", "add", "omlx", "--base-url", "http://localhost:8000/v1", "--kind", "systemone"}, io.Discard, io.Discard); code != 0 {
		t.Fatal("add failed")
	}
	c, e := loadConfig()
	if e != nil || providerKind(allProviders(c)["omlx"]) != "systemone" {
		t.Fatal(c, e)
	}
	if e := validateSetting("eval.kind", "other"); e == nil {
		t.Fatal("invalid kind accepted")
	}
	if e := validateSetting("eval.options", "other"); e == nil {
		t.Fatal("invalid options accepted")
	}
	if e := validateSetting("eval.max-options", "0"); e == nil {
		t.Fatal("zero cap accepted")
	}
}
