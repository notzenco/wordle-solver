package main

import (
	"encoding/json"
	"testing"
)

// --json output must be valid JSON with stable field names, so scripts
// don't break when the human-readable tables change.
func TestJSONOutput(t *testing.T) {
	decode := func(args ...string) any {
		t.Helper()
		code, out, errOut := run(t, args...)
		if code != 0 {
			t.Fatalf("%v: code %d, %s", args, code, errOut)
		}
		var v any
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatalf("%v: not JSON: %v\n%s", args, err, out)
		}
		return v
	}
	runs := decode("bench", "--strategy", "freq", "--json").([]any)
	if r := runs[0].(map[string]any); r["strategy"] != "freq" || r["games"].(float64) != 2315 || r["failed"].(float64) != 22 {
		t.Errorf("bench --strategy: %v", r)
	}
	if r := decode("bench", "--n", "20", "--seed", "1", "--json").(map[string]any); r["games"].(float64) != 20 {
		t.Errorf("bench: %v", r)
	}
	if r := decode("stats", "--json").(map[string]any); r["played"].(float64) != 0 {
		t.Errorf("stats: %v", r)
	}
	found := false
	for _, p := range decode("providers", "--json").([]any) {
		if p := p.(map[string]any); p["name"] == "ollama" && p["base_url"] != "" && p["source"] == "built-in" {
			found = true
		}
	}
	if !found {
		t.Error("providers --json lacks the built-in ollama")
	}
	found = false
	for _, s := range decode("settings", "--json").([]any) {
		if s := s.(map[string]any); s["key"] == "max" && s["value"] == "6" && s["source"] == "default" {
			found = true
		}
	}
	if !found {
		t.Error("settings --json lacks max=6 from its default")
	}
	if code, _, _ := run(t, "providers", "check", "ollama", "--json"); code != 2 {
		t.Errorf("--json applies only to list; got code %d", code)
	}
}
