package schedule

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestLaunchdPlist(t *testing.T) {
	plist := launchdPlist("/Users/me/.local/bin/wordle-solver", 0, 5, "/Users/me/Library/Logs/wordle-solver.log",
		map[string]string{"WORDLE_SOLVER_HOME": "/tmp/home & co"})
	var doc struct{ XMLName xml.Name }
	if err := xml.Unmarshal([]byte(plist), &doc); err != nil {
		t.Fatalf("not valid XML: %v\n%s", err, plist)
	}
	for _, want := range []string{
		"<string>" + launchdLabel + "</string>",
		"<string>/Users/me/.local/bin/wordle-solver</string>",
		"<string>-catchup</string>",
		"<key>Hour</key>\n\t\t<integer>0</integer>",
		"<key>Minute</key>\n\t\t<integer>5</integer>",
		"<string>/tmp/home &amp; co</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("plist lacks %q:\n%s", want, plist)
		}
	}
}

func TestSystemdUnits(t *testing.T) {
	service, timer := systemdUnits("/home/me/my bin/wordle-solver", 7, 30, "/home/me/.local/state/wordle-solver/wordle-solver.log",
		map[string]string{"WORDLE_SOLVER_HOME": "/tmp/100% $home"})
	for _, want := range []string{
		`ExecStart="/home/me/my bin/wordle-solver" auto -catchup -log /home/me/.local/state/wordle-solver/wordle-solver.log`,
		`Environment="WORDLE_SOLVER_HOME=/tmp/100%% $home"`,
		"Type=oneshot",
	} {
		if !strings.Contains(service, want) {
			t.Errorf("service lacks %q:\n%s", want, service)
		}
	}
	for _, want := range []string{"OnCalendar=*-*-* 07:30:00", "Persistent=true", "WantedBy=timers.target"} {
		if !strings.Contains(timer, want) {
			t.Errorf("timer lacks %q:\n%s", want, timer)
		}
	}
}

func TestWindowsTask(t *testing.T) {
	task := windowsTask(`C:\Users\Me\bin\wordle-solver.exe`, 0, 5, `C:\Users\Me\AppData\Local\wordle-solver\wordle-solver.log`,
		map[string]string{"WORDLE_SOLVER_HOME": `D:\Wordle & Co\`})
	var doc struct{ XMLName xml.Name }
	if err := xml.Unmarshal([]byte(strings.Replace(task, `encoding="UTF-16"`, `encoding="UTF-8"`, 1)), &doc); err != nil {
		t.Fatalf("not valid XML: %v\n%s", err, task)
	}
	for _, want := range []string{
		"<StartBoundary>2000-01-01T00:05:00</StartBoundary>",
		"<StartWhenAvailable>true</StartWhenAvailable>",
		`<Command>C:\Users\Me\bin\wordle-solver.exe</Command>`,
		`<Arguments>auto -catchup -log C:\Users\Me\AppData\Local\wordle-solver\wordle-solver.log -home "D:\Wordle &amp; Co\\"</Arguments>`,
	} {
		if !strings.Contains(task, want) {
			t.Errorf("task lacks %q:\n%s", want, task)
		}
	}
}
