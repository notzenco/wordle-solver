package main

import (
	"flag"
	"github.com/notzenco/wordle-solver/internal/player"
	"io"
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
	src, err := applySettings(fs, []*option{optionByID("max")}, c)
	if err != nil || *v != 5 || src["max"] != "env" {
		t.Fatal(src, err, *v)
	}
	fs.Parse([]string{"--max=3"})
	src, err = applySettings(fs, []*option{optionByID("max")}, c)
	if err != nil || *v != 3 || src["max"] != "flag" {
		t.Fatal(src, err)
	}
	if err := validateSetting("unknown", "x"); err == nil {
		t.Fatal("unknown accepted")
	}
	if err := validateSetting("max", "99"); err == nil {
		t.Fatal("bad max")
	}
	if code := runCLI([]string{"settings", "reset", "max"}, io.Discard, io.Discard); code != 0 {
		t.Fatal("reset failed")
	}
	c, _ = loadConfig()
	if _, ok := c.Settings["max"]; ok {
		t.Fatal("not reset")
	}
}

func TestSavedDefaultsAndSet(t *testing.T) {
	t.Setenv("WORDLE_SOLVER_HOME", t.TempDir())
	c, _ := loadConfig()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	v := fs.Int("max", 6, "")
	src, e := applySettings(fs, []*option{optionByID("max")}, c)
	if e != nil || *v != 6 || src["max"] != "default" {
		t.Fatal(src, e)
	}
	if code := runCLI([]string{"settings", "set", "max", "4"}, io.Discard, io.Discard); code != 0 {
		t.Fatal("set failed")
	}
	c, e = loadConfig()
	if e != nil {
		t.Fatal(e)
	}
	src, e = applySettings(fs, []*option{optionByID("max")}, c)
	if e != nil || *v != 4 || src["max"] != "config" {
		t.Fatal(src, e, *v)
	}
}
