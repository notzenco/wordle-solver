// Package schedule sets up the daily run with each platform's own scheduler:
// launchd on macOS, a systemd user timer on Linux and Task Scheduler on
// Windows. Nothing is scheduled until Install is called.
package schedule

import (
	"encoding/binary"
	"fmt"
	"html"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode/utf16"
)

const (
	launchdLabel    = "wordle-solver.daily"
	systemdName     = "wordle-solver"
	windowsTaskName = "wordle-solver daily"
)

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

// systemdUnits describes a user service and a timer that starts it daily
// at hour:minute. Persistent=true makes a start missed while the machine
// was off happen as soon as it is back on.
func systemdUnits(exe string, hour, minute int, logPath string, env map[string]string) (service, timer string) {
	var b strings.Builder
	fmt.Fprintf(&b, "[Unit]\nDescription=Play the daily Wordle\n\n[Service]\nType=oneshot\nExecStart=%s auto -catchup -log %s\n",
		systemdQuote(exe, true), systemdQuote(logPath, true))
	for _, k := range slices.Sorted(maps.Keys(env)) {
		// Environment= expands % specifiers but, unlike ExecStart=, not $.
		fmt.Fprintf(&b, "Environment=%s\n", systemdQuote(k+"="+env[k], false))
	}
	timer = fmt.Sprintf("[Unit]\nDescription=Play the daily Wordle\n\n[Timer]\nOnCalendar=*-*-* %02d:%02d:00\nPersistent=true\n\n[Install]\nWantedBy=timers.target\n",
		hour, minute)
	return b.String(), timer
}

func systemdQuote(s string, dollar bool) string {
	s = strings.ReplaceAll(s, "%", "%%")
	if dollar {
		s = strings.ReplaceAll(s, "$", "$$")
	}
	if !strings.ContainsAny(s, " \t\"'\\;") {
		return s
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// windowsTask describes a Task Scheduler task that runs daily at
// hour:minute and, with StartWhenAvailable, as soon as possible after a
// missed start. Tasks cannot set environment variables, so a custom data
// folder is passed as -home.
func windowsTask(exe string, hour, minute int, logPath string, env map[string]string) string {
	args := []string{"auto", "-catchup", "-log", windowsQuote(logPath)}
	if d := env["WORDLE_SOLVER_HOME"]; d != "" {
		args = append(args, "-home", windowsQuote(d))
	}
	esc := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Play the daily Wordle</Description>
  </RegistrationInfo>
  <Triggers>
    <CalendarTrigger>
      <StartBoundary>2000-01-01T%02d:%02d:00</StartBoundary>
      <ScheduleByDay>
        <DaysInterval>1</DaysInterval>
      </ScheduleByDay>
    </CalendarTrigger>
  </Triggers>
  <Settings>
    <StartWhenAvailable>true</StartWhenAvailable>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <ExecutionTimeLimit>PT1H</ExecutionTimeLimit>
  </Settings>
  <Actions>
    <Exec>
      <Command>%s</Command>
      <Arguments>%s</Arguments>
    </Exec>
  </Actions>
</Task>
`, hour, minute, esc(exe), esc(strings.Join(args, " ")))
}

// windowsQuote quotes s as one argument the way Windows programs parse
// command lines, where backslashes are literal except before a quote.
func windowsQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, c := range s {
		switch c {
		case '\\':
			slashes++
		case '"':
			b.WriteString(strings.Repeat(`\`, 2*slashes+1))
			slashes = 0
		default:
			slashes = 0
		}
		b.WriteRune(c)
	}
	b.WriteString(strings.Repeat(`\`, slashes))
	b.WriteByte('"')
	return b.String()
}

// LogPath is where the daily run writes its output.
func LogPath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		return filepath.Join(home, "Library", "Logs", "wordle-solver.log"), err
	case "windows":
		base, err := os.UserCacheDir() // %LocalAppData%
		return filepath.Join(base, "wordle-solver", "wordle-solver.log"), err
	default:
		base := os.Getenv("XDG_STATE_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "state")
		}
		return filepath.Join(base, "wordle-solver", "wordle-solver.log"), nil
	}
}

func systemdDir() (string, error) {
	base, err := os.UserConfigDir()
	return filepath.Join(base, "systemd", "user"), err
}

func launchdPath() (string, error) {
	home, err := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), err
}

// Installed reports whether the daily run is set up.
func Installed() bool {
	switch runtime.GOOS {
	case "darwin":
		p, err := launchdPath()
		return err == nil && exists(p)
	case "windows":
		return exec.Command("schtasks", "/Query", "/TN", windowsTaskName).Run() == nil
	default:
		dir, err := systemdDir()
		return err == nil && exists(filepath.Join(dir, systemdName+".timer"))
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Install sets up the platform's own scheduler to run "exe auto -catchup"
// daily at hour:minute local time, replacing any earlier daily run, and
// returns the log the run writes to. env is passed to the run.
func Install(exe string, hour, minute int, env map[string]string) (logPath string, err error) {
	log, err := LogPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(log), 0o755); err != nil {
		return "", err
	}
	if err := Remove(); err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin":
		path, err := launchdPath()
		if err != nil {
			return "", err
		}
		if err := writeFile(path, launchdPlist(exe, hour, minute, log, env)); err != nil {
			return "", err
		}
		return log, run("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), path)
	case "windows":
		path := filepath.Join(os.TempDir(), "wordle-solver-task.xml")
		if err := writeFile(path, utf16File(windowsTask(exe, hour, minute, log, env))); err != nil {
			return "", err
		}
		defer os.Remove(path)
		return log, run("schtasks", "/Create", "/TN", windowsTaskName, "/XML", path, "/F")
	case "linux":
		if _, err := exec.LookPath("systemctl"); err != nil {
			return "", fmt.Errorf("schedule needs systemd; instead, add a cron job that runs: %s auto -catchup -log %s", exe, log)
		}
		dir, err := systemdDir()
		if err != nil {
			return "", err
		}
		service, timer := systemdUnits(exe, hour, minute, log, env)
		if err := writeFile(filepath.Join(dir, systemdName+".service"), service); err != nil {
			return "", err
		}
		if err := writeFile(filepath.Join(dir, systemdName+".timer"), timer); err != nil {
			return "", err
		}
		if err := run("systemctl", "--user", "daemon-reload"); err != nil {
			return "", err
		}
		return log, run("systemctl", "--user", "enable", "--now", systemdName+".timer")
	default:
		return "", fmt.Errorf("schedule isn't supported on %s; instead, run '%s auto -catchup -log %s' daily from cron", runtime.GOOS, exe, log)
	}
}

// Remove takes the daily run out of the scheduler, if it is there.
func Remove() error {
	switch runtime.GOOS {
	case "darwin":
		// Unloading fails harmlessly when the job isn't loaded.
		exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), launchdLabel)).Run()
		path, err := launchdPath()
		if err != nil {
			return err
		}
		return removeFile(path)
	case "windows":
		if Installed() {
			return run("schtasks", "/Delete", "/TN", windowsTaskName, "/F")
		}
	case "linux":
		dir, err := systemdDir()
		if err != nil {
			return err
		}
		timer := filepath.Join(dir, systemdName+".timer")
		if !exists(timer) {
			return nil
		}
		exec.Command("systemctl", "--user", "disable", "--now", systemdName+".timer").Run()
		if err := removeFile(timer); err != nil {
			return err
		}
		if err := removeFile(filepath.Join(dir, systemdName+".service")); err != nil {
			return err
		}
		return run("systemctl", "--user", "daemon-reload")
	}
	return nil
}

// utf16File encodes an XML document as UTF-16 with a byte-order mark, the
// encoding schtasks expects.
func utf16File(s string) string {
	units := utf16.Encode([]rune(strings.ReplaceAll(s, "\n", "\r\n")))
	b := binary.LittleEndian.AppendUint16(nil, 0xFEFF)
	for _, u := range units {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return string(b)
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func run(name string, args ...string) error {
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, out)
	}
	return nil
}
