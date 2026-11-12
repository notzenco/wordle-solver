package main

import (
	"fmt"
	"html"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const launchdLabel = "wordle-solver.daily"

// launchdPlist describes a job that runs "exe auto -catchup" daily at
// hour:minute local time. launchd runs a missed job once the Mac wakes.
func launchdPlist(exe string, hour, minute int, logPath string, env map[string]string) string {
	var b strings.Builder
	esc := html.EscapeString
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>auto</string>
		<string>-catchup</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>%d</integer>
		<key>Minute</key>
		<integer>%d</integer>
	</dict>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
`, launchdLabel, esc(exe), hour, minute, esc(logPath), esc(logPath))
	if len(env) > 0 {
		b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
		for _, k := range slices.Sorted(maps.Keys(env)) {
			fmt.Fprintf(&b, "\t\t<key>%s</key>\n\t\t<string>%s</string>\n", esc(k), esc(env[k]))
		}
		b.WriteString("\t</dict>\n")
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
}

// schedule installs (or with remove, uninstalls) the daily launchd job.
func schedule(at string, remove bool) {
	if runtime.GOOS != "darwin" {
		fail("schedule uses launchd, so it needs macOS; elsewhere run 'wordle-solver auto -catchup' from cron")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err.Error())
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	// Unload any earlier version first; it is fine if none is loaded.
	exec.Command("launchctl", "bootout", domain+"/"+launchdLabel).Run()

	if remove {
		if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
			fail(err.Error())
		}
		fmt.Println("Daily run removed.")
		return
	}

	var hour, minute int
	if _, err := fmt.Sscanf(at, "%d:%d", &hour, &minute); err != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		fail("-at must look like 00:05")
	}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		fail(err.Error())
	}
	env := map[string]string{}
	if d := os.Getenv("WORDLE_SOLVER_HOME"); d != "" {
		env["WORDLE_SOLVER_HOME"] = d
	}
	logPath := filepath.Join(home, "Library", "Logs", "wordle-solver.log")
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(plistPath, []byte(launchdPlist(exe, hour, minute, logPath, env)), 0o644); err != nil {
		fail(err.Error())
	}
	if out, err := exec.Command("launchctl", "bootstrap", domain, plistPath).CombinedOutput(); err != nil {
		fail(fmt.Sprintf("launchctl bootstrap: %v: %s", err, out))
	}
	fmt.Printf("Daily run installed: %s plays at %02d:%02d local time and catches up on missed days.\n", filepath.Base(exe), hour, minute)
	fmt.Printf("Log: %s\nRemove with: wordle-solver schedule -remove\n", logPath)
}
