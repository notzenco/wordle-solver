package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if code := runCLI([]string{"providers", "add", "ollama", "--base-url", s.URL}, io.Discard, io.Discard); code != 0 {
		t.Fatal("add failed")
	}
	c, _ = loadConfig()
	p := allProviders(c)["ollama"]
	m, e := fetchModels(p)
	if e != nil || len(m) != 1 || m[0] != "a/b" {
		t.Fatal(m, e)
	}
	if code := runCLI([]string{"providers", "remove", "ollama"}, io.Discard, io.Discard); code != 0 {
		t.Fatal("remove failed")
	}
	c, _ = loadConfig()
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
	m, e := fetchModels(provider{s.URL, "PRIVATE_API_KEY", ""})
	if e != nil || len(m) != 1 || m[0] != "[redacted]" {
		t.Fatal("key redaction failed")
	}
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	if e := saveConfig(configFile{Providers: map[string]provider{"test": {s.URL, "PRIVATE_API_KEY", ""}}}); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if code := runCLI([]string{"providers", "check", "test"}, &out, io.Discard); code != 0 || strings.Contains(out.String(), "secret-value") {
		t.Fatalf("check failed or leaked the key: %q", out.String())
	}
}
