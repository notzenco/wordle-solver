package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/notzenco/wordle-solver/internal/player"
	"github.com/notzenco/wordle-solver/internal/wordle"
)

type configFile struct {
	Providers map[string]provider `json:"providers,omitempty"`
	Settings  map[string]string   `json:"settings,omitempty"`
}

var settingFlags = map[string]string{
	"max": "max", "sloppy": "sloppy", "hard": "hard", "target": "target", "strategy": "strategy", "copy": "copy",
	"eval.model": "model", "eval.games": "games", "eval.set": "set", "eval.seed": "seed",
	"eval.temperature": "temperature", "eval.max-tokens": "max-tokens", "eval.timeout": "timeout", "eval.out": "out",
	"eval.base-url": "base-url", "eval.api-key-env": "api-key-env",
}

func configPath() (string, error) { d, e := player.Dir(); return filepath.Join(d, "config.json"), e }
func loadConfig() (configFile, error) {
	c := configFile{Settings: map[string]string{}}
	p, e := configPath()
	if e != nil {
		return c, e
	}
	b, e := os.ReadFile(p)
	if errors.Is(e, os.ErrNotExist) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if c.Settings == nil {
		c.Settings = map[string]string{}
	}
	for n, p := range c.Providers {
		if !providerName.MatchString(n) {
			return c, fmt.Errorf("invalid provider name in config")
		}
		if err := validBase(p.BaseURL); err != nil {
			return c, err
		}
		if err := validKeyEnv(p.KeyEnv); err != nil {
			return c, err
		}
	}
	for k, v := range c.Settings {
		if err := validateSetting(k, v); err != nil {
			return c, err
		}
	}
	return c, e
}
func saveConfig(c configFile) error {
	p, e := configPath()
	if e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), "config-*.tmp")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(append(b, '\n')); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), p)
}
func validateSetting(k, v string) error {
	if _, ok := settingFlags[k]; !ok {
		return fmt.Errorf("unknown setting %q", k)
	}
	bad := func() error { return fmt.Errorf("invalid value for %s", k) }
	switch k {
	case "hard", "copy":
		_, e := strconv.ParseBool(v)
		return e
	case "max", "target", "eval.games", "eval.max-tokens", "eval.seed":
		n, e := strconv.ParseUint(v, 10, 64)
		if e != nil {
			return bad()
		}
		if k == "max" && (n < 3 || n > 6) || k == "target" && n != 0 && (n < 2 || n > 6) || (k == "eval.games" || k == "eval.max-tokens") && n == 0 {
			return bad()
		}
	case "sloppy", "eval.temperature":
		n, e := strconv.ParseFloat(v, 64)
		if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || k == "sloppy" && n > 1 {
			return bad()
		}
	case "strategy":
		if v != "" && v != "all" {
			if _, ok := wordle.StrategyByName(v); !ok {
				return bad()
			}
		}
	case "eval.base-url":
		if v != "" {
			return validBase(v)
		}
	case "eval.api-key-env":
		return validKeyEnv(v)
	case "eval.set":
		if v != "answers" && v != "heldout" {
			return bad()
		}
	case "eval.timeout":
		d, e := time.ParseDuration(v)
		if e != nil || d <= 0 {
			return bad()
		}
	}
	return nil
}
func applySettings(fs *flag.FlagSet, c configFile) (map[string]string, error) {
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	src := map[string]string{}
	for k, name := range settingFlags {
		f := fs.Lookup(name)
		if f == nil {
			continue
		}
		v, ok := c.Settings[k]
		source := "default"
		if ok {
			source = "config"
		} else {
			v = f.DefValue
		}
		env := "WORDLE_SOLVER_" + strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(k))
		if x, ok := os.LookupEnv(env); ok {
			v = x
			source = "env"
		}
		if explicit[name] {
			v = f.Value.String()
			source = "flag"
		}
		if e := validateSetting(k, v); e != nil {
			return nil, e
		}
		if e := f.Value.Set(v); e != nil {
			return nil, e
		}
		src[k] = source
	}
	return src, nil
}
func settingsCommand(args []string, fs *flag.FlagSet, c configFile, src map[string]string) error {
	if len(args) == 0 {
		keys := []string{}
		for k := range src {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s\t%s\t%s\n", k, fs.Lookup(settingFlags[k]).Value.String(), src[k])
		}
		return nil
	}
	if len(args) == 3 && args[0] == "set" {
		if e := validateSetting(args[1], args[2]); e != nil {
			return e
		}
		c.Settings[args[1]] = args[2]
	} else if len(args) == 2 && args[0] == "reset" {
		if _, ok := settingFlags[args[1]]; !ok {
			return fmt.Errorf("unknown setting %q", args[1])
		}
		delete(c.Settings, args[1])
	} else {
		return fmt.Errorf("usage: settings [set KEY VALUE | reset KEY]")
	}
	return saveConfig(c)
}
