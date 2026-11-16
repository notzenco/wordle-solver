package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateCheckOnlyReports(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v9.0.0","assets":[]}`))
	}))
	defer s.Close()
	old := latestURL
	latestURL = s.URL
	oldVersion := version
	version = "v0.1.0"
	defer func() { latestURL, version = old, oldVersion }()
	code, out, errOut := run(t, "update", "--check")
	if code != 0 || !strings.Contains(out, "v9.0.0") || !strings.Contains(out, "update") {
		t.Fatalf("code %d\n%s\n%s", code, out, errOut)
	}
	version = "dev"
	if _, out, _ := run(t, "update", "--check"); !strings.Contains(out, "development build") {
		t.Fatalf("dev build: %s", out)
	}
	if code, _, _ := run(t, "update", "--check", "--force"); code != 2 {
		t.Fatalf("--check with --force: code %d", code)
	}
}
