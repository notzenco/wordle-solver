package main

import (
	"bytes"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	code := runCLI(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestHelpListsEveryCommand(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		code, out, _ := run(t, args...)
		if code != 0 {
			t.Fatalf("%v exited %d", args, code)
		}
		for _, c := range commands {
			if !strings.Contains(out, "  "+c.name+" ") {
				t.Errorf("%v: help lacks %q", args, c.name)
			}
		}
	}
}

func TestCommandHelpShowsOnlyItsOptions(t *testing.T) {
	for _, args := range [][]string{{"help", "eval"}, {"eval", "-h"}, {"eval", "--help"}} {
		code, out, _ := run(t, args...)
		if code != 0 {
			t.Fatalf("%v exited %d", args, code)
		}
		for _, want := range []string{"Usage:", "--model PROVIDER/MODEL", "--kind", "eval.model", "Examples:"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: help lacks %q:\n%s", args, want, out)
			}
		}
		for _, unwanted := range []string{"--sloppy", "--date", "--strategy"} {
			if strings.Contains(out, unwanted) {
				t.Errorf("%v: eval help shows %s", args, unwanted)
			}
		}
	}
}

func TestOptionsAreScopedToTheirCommand(t *testing.T) {
	code, _, errOut := run(t, "play", "--games", "3")
	if code != 2 || !strings.Contains(errOut, "-games") || !strings.Contains(errOut, "wordle-solver help play") {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
}

func TestOptionsMayFollowArguments(t *testing.T) {
	code, out, errOut := run(t, "sim", "crane", "--hard", "--seed", "1")
	if code != 0 || !strings.Contains(out, "CRANE") {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out, errOut)
	}
}

func TestUnknownCommandSuggestsClosest(t *testing.T) {
	code, _, errOut := run(t, "provders")
	if code != 2 || !strings.Contains(errOut, `did you mean "providers"`) {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
}

func TestSettingsListHasHeaderAndDescriptions(t *testing.T) {
	code, out, _ := run(t, "settings")
	if code != 0 || !strings.HasPrefix(out, "KEY") || !strings.Contains(out, "eval.model") || !strings.Contains(out, "default") {
		t.Fatalf("code %d:\n%s", code, out)
	}
	code, out, _ = run(t, "settings", "get", "max")
	if code != 0 || strings.TrimSpace(out) != "6" {
		t.Fatalf("get max: code %d, %q", code, out)
	}
}

func TestProvidersTableShowsNoneForKeylessServers(t *testing.T) {
	code, out, _ := run(t, "providers")
	if code != 0 || !strings.HasPrefix(out, "NAME") {
		t.Fatalf("code %d:\n%s", code, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "ollama ") && !strings.Contains(line, " none ") {
			t.Errorf("ollama row should show no key: %q", line)
		}
		if strings.HasPrefix(line, "openai ") && !strings.Contains(line, "OPENAI_API_KEY") {
			t.Errorf("openai row should name its key variable: %q", line)
		}
	}
}

func TestEvalNeedsAModel(t *testing.T) {
	code, _, errOut := run(t, "eval")
	if code != 2 || !strings.Contains(errOut, "--model") {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
}
