package main

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
