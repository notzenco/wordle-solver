package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviders(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	c, _ := loadConfig()
	for _, v := range []string{"sk-secret", "abc secret", "TOKEN=secret"} {
		if validKeyEnv(v) == nil {
			t.Fatal(v)
		}
	}
	for _, v := range []string{"ftp://host", "https://user:pass@host", "https://host?key=secret"} {
		if validBase(v) == nil {
			t.Fatal(v)
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Error(r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"id":"a/b"}]}`))
	}))
	defer s.Close()
	if e := providersCommand([]string{"add", "ollama", "--base-url", s.URL}, &c); e != nil {
		t.Fatal(e)
	}
	p := allProviders(c)["ollama"]
	m, e := fetchModels(p)
	if e != nil || len(m) != 1 || m[0] != "a/b" {
		t.Fatal(m, e)
	}
	if e := providersCommand([]string{"remove", "ollama"}, &c); e != nil {
		t.Fatal(e)
	}
	if allProviders(c)["ollama"].BaseURL != builtins["ollama"].BaseURL {
		t.Fatal("override remained")
	}
}

func TestModelKeyRedaction(t *testing.T) {
	t.Setenv("PRIVATE_API_KEY", "secret-value")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"secret-value"}]}`))
	}))
	defer s.Close()
	m, e := fetchModels(provider{s.URL, "PRIVATE_API_KEY"})
	if e != nil || len(m) != 1 || m[0] != "[redacted]" {
		t.Fatal("key redaction failed")
	}
	c := configFile{Providers: map[string]provider{"test": {s.URL, "PRIVATE_API_KEY"}}}
	if e := providersCommand([]string{"check", "test"}, &c); e != nil {
		t.Fatal(e)
	}
}
