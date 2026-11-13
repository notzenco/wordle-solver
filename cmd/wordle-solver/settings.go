package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/notzenco/wordle-solver/internal/player"
	"github.com/notzenco/wordle-solver/internal/wordle"
)

type configFile struct {
	Providers map[string]provider `json:"providers,omitempty"`
	Settings  map[string]string   `json:"settings,omitempty"`
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
	if e = json.Unmarshal(b, &c); e != nil {
		return c, fmt.Errorf("reading %s: %v", p, e)
	}
	if c.Settings == nil {
		c.Settings = map[string]string{}
	}
	for n, p := range c.Providers {
		if !providerName.MatchString(n) {
			return c, fmt.Errorf("invalid provider name in config")
		}
		if err := validKind(p.Kind); err != nil {
			return c, err
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
	return c, nil
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
	if o := optionByID(k); o == nil || !o.setting {
		return fmt.Errorf("unknown setting %q; run 'wordle-solver help settings' for the list", k)
	}
	bad := func() error { return fmt.Errorf("invalid value %q for %s", v, k) }
	switch k {
	case "eval.kind":
		return validKind(v)
	case "eval.options":
		if v != "" && v != "free" && v != "candidates" {
			return bad()
		}
	case "eval.max-options", "eval.distractors":
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 || k == "eval.max-options" && n == 0 {
			return bad()
		}
	case "hard", "copy":
		if _, e := strconv.ParseBool(v); e != nil {
			return bad()
		}
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

func settingEnv(key string) string {
	return "WORDLE_SOLVER_" + strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(key))
}

// applySettings fills each setting option in fs from flag, env, config or
// default, in that order, and returns where each value came from.
func applySettings(fs *flag.FlagSet, opts []*option, c configFile) (map[string]string, error) {
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	src := map[string]string{}
	for _, o := range opts {
		if !o.setting {
			continue
		}
		f := fs.Lookup(o.flag)
		if f == nil {
			continue
		}
		v, ok := c.Settings[o.id]
		source := "default"
		if ok {
			source = "config"
		} else {
			v = f.DefValue
		}
		if x, ok := os.LookupEnv(settingEnv(o.id)); ok {
			v, source = x, "env"
		}
		if explicit[o.flag] {
			v, source = f.Value.String(), "flag"
		}
		if e := validateSetting(o.id, v); e != nil {
			switch source {
			case "flag":
				return nil, usageError(fmt.Sprintf("--%s: %v", o.flag, e))
			case "env":
				return nil, fmt.Errorf("%s: %v", settingEnv(o.id), e)
			case "config":
				return nil, fmt.Errorf("saved setting %v; fix it with 'wordle-solver settings set' or 'reset'", e)
			}
			return nil, e
		}
		if e := f.Value.Set(v); e != nil {
			return nil, e
		}
		src[o.id] = source
	}
	return src, nil
}

func settingsCommand(c *cmdCtx) error {
	sub := "list"
	args := c.args
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	if c.bool("all") && sub != "reset" {
		return usageError("--all only applies to reset")
	}
	// Several settings share an option name across commands (eval.model and
	// play.model are both --model), so this listing names flags by key.
	opts := []*option{}
	for _, o := range settingOptions() {
		byKey := *o
		byKey.flag = o.id
		opts = append(opts, &byKey)
	}
	fs := newFlagSet("settings", opts)
	src, err := applySettings(fs, opts, c.config)
	if err != nil {
		return err
	}
	value := func(k string) string {
		if f := fs.Lookup(k); f != nil {
			return f.Value.String()
		}
		return ""
	}
	switch {
	case sub == "list" && len(args) == 0:
		tw := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE")
		for _, o := range opts {
			v := value(o.id)
			if v == "" {
				v = "-"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", o.id, v, src[o.id])
		}
		return tw.Flush()
	case sub == "get" && len(args) == 1:
		if err := validateSetting(args[0], value(args[0])); err != nil {
			return err
		}
		fmt.Fprintln(stdout, value(args[0]))
		return nil
	case sub == "set" && len(args) == 2:
		if err := validateSetting(args[0], args[1]); err != nil {
			return err
		}
		c.config.Settings[args[0]] = args[1]
		if err := saveConfig(c.config); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Saved %s = %s\n", args[0], args[1])
		if _, ok := os.LookupEnv(settingEnv(args[0])); ok {
			fmt.Fprintf(stdout, "Note: %s is set in your environment and takes precedence.\n", settingEnv(args[0]))
		}
		return nil
	case sub == "reset" && c.bool("all") && len(args) == 0:
		c.config.Settings = map[string]string{}
		if err := saveConfig(c.config); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Reset every saved setting to its default.")
		return nil
	case sub == "reset" && len(args) == 1:
		if o := optionByID(args[0]); o == nil || !o.setting {
			return fmt.Errorf("unknown setting %q; run 'wordle-solver help settings' for the list", args[0])
		}
		delete(c.config.Settings, args[0])
		if err := saveConfig(c.config); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Reset %s to its default.\n", args[0])
		return nil
	}
	return usageError("usage: wordle-solver settings [list | get KEY | set KEY VALUE | reset KEY | reset --all]")
}
