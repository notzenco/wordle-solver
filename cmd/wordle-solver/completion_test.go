package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompletionScriptsCoverEveryCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		code, out, errOut := run(t, "completion", shell)
		if code != 0 {
			t.Fatalf("%s: code %d, %s", shell, code, errOut)
		}
		for _, c := range commands {
			if !strings.Contains(out, c.name) {
				t.Errorf("%s script lacks command %s", shell, c.name)
			}
		}
		for _, flag := range []string{"strategy", "max-options", "offline"} {
			if !strings.Contains(out, flag) {
				t.Errorf("%s script lacks --%s", shell, flag)
			}
		}
		if !strings.Contains(out, "wsolve") {
			t.Errorf("%s script doesn't cover the wsolve name", shell)
		}
		// Each script must at least parse in its shell, where installed.
		if path, err := exec.LookPath(shell); err == nil && runtime.GOOS != "windows" {
			f := filepath.Join(t.TempDir(), "script")
			os.WriteFile(f, []byte(out), 0o600)
			if b, err := exec.Command(path, "-n", f).CombinedOutput(); err != nil {
				t.Errorf("%s -n: %v\n%s", shell, err, b)
			}
		}
	}
	if code, _, _ := run(t, "completion", "tcsh"); code != 2 {
		t.Errorf("unknown shell: code %d", code)
	}
}
