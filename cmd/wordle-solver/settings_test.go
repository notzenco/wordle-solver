package main

import (
	"flag"
	"github.com/notzenco/wordle-solver/internal/player"
	"testing"
)

func TestSettings(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	d, _ := player.Dir()
	if d == "" {
		t.Fatal("missing home")
	}
	c := configFile{Settings: map[string]string{"max": "4"}}
	if err := saveConfig(c); err != nil {
		t.Fatal(err)
	}
	c, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	v := fs.Int("max", 6, "")
	t.Setenv("WORDLE_SOLVER_MAX", "5")
	src, err := applySettings(fs, c)
	if err != nil || *v != 5 || src["max"] != "env" {
		t.Fatal(src, err, *v)
	}
	fs.Parse([]string{"--max=3"})
	src, err = applySettings(fs, c)
	if err != nil || *v != 3 || src["max"] != "flag" {
		t.Fatal(src, err)
	}
	if err := validateSetting("unknown", "x"); err == nil {
		t.Fatal("unknown accepted")
	}
	if err := validateSetting("max", "99"); err == nil {
		t.Fatal("bad max")
	}
	if err := settingsCommand([]string{"reset", "max"}, fs, c, src); err != nil {
		t.Fatal(err)
	}
	c, _ = loadConfig()
	if _, ok := c.Settings["max"]; ok {
		t.Fatal("not reset")
	}
}
