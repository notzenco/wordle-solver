package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

// stdout and stderr are replaced in tests; commands write through them.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
	stdin  io.Reader // nil means os.Stdin
)

// An option is one command-line flag. Options with setting set can also come
// from WORDLE_SOLVER_<KEY>, the saved config or their default, in that order
// after the flag itself; the id is then the setting key.
type option struct {
	id      string
	flag    string
	arg     string // placeholder in help; empty for booleans
	def     string
	typ     string // bool, int, uint, float, string or duration
	usage   string
	setting bool
}

var allOptions = []option{
	{"max", "max", "N", "6", "int", "plan to solve within N guesses (3-6)", true},
	{"target", "target", "N", "0", "int", "aim to finish on turn N (2-6; 0 lets the player model choose)", true},
	{"hard", "hard", "", "false", "bool", "only make guesses Hard Mode allows", true},
	{"sloppy", "sloppy", "P", "0.2", "float", "chance of a human-like slip per guess while turns are spare (0-1)", true},
	{"copy", "copy", "", "false", "bool", "copy the share grid to the clipboard", true},
	{"seed", "seed", "N", "0", "uint", "seed random choices so a run repeats (0 picks a random seed)", false},
	{"dry", "dry", "", "false", "bool", "neither read nor save player data", false},

	{"date", "date", "YYYY-MM-DD", "", "string", "solve the puzzle published on this date", false},
	{"id", "id", "N", "-1", "int", "solve puzzle number N (0 is 2021-06-19)", false},
	{"from", "from", "N", "-1", "int", "solve every puzzle from number N", false},
	{"to", "to", "N", "-1", "int", "stop a --from range at puzzle N (default: today's)", false},
	{"fresh", "fresh", "", "false", "bool", "replay days already played and replace their records", false},

	{"n", "n", "N", "0", "int", "play N random answers instead of all of them", false},
	{"strategy", "strategy", "NAME", "", "string", "benchmark a guess strategy instead of the player model; NAME is one of the strategies below or all", true},

	{"y", "y", "", "false", "bool", "delete without asking", false},
	{"force", "force", "", "false", "bool", "reinstall even when already up to date", false},

	{"play.model", "model", "PROVIDER/MODEL", "", "string", "take suggestions from this model instead of the built-in player; decision models such as Clef choose from the words that fit", true},

	{"eval.model", "model", "PROVIDER/MODEL", "", "string", "model to evaluate; the provider is the text before the first slash", true},
	{"eval.games", "games", "N", "10", "uint", "number of games", true},
	{"eval.set", "set", "SET", "answers", "string", "answers (the official list) or heldout (valid words never used as answers)", true},
	{"eval.seed", "seed", "N", "0", "uint", "seed for the answer sample and offered options", true},
	{"eval.kind", "kind", "KIND", "", "string", "chat or systemone (default: the provider's kind, switching to systemone when the server says the model is a decision model)", true},
	{"eval.options", "options", "MODE", "", "string", "free or candidates (default: free for chat, candidates for systemone)", true},
	{"eval.max-options", "max-options", "N", "64", "uint", "most consistent words offered per turn in candidates mode", true},
	{"eval.distractors", "distractors", "N", "0", "uint", "inconsistent valid words added to the offered options, to test rule-following", true},
	{"eval.temperature", "temperature", "T", "0", "float", "sampling temperature (chat only)", true},
	{"eval.max-tokens", "max-tokens", "N", "64", "uint", "reply token limit (chat only)", true},
	{"eval.timeout", "timeout", "DURATION", "1m0s", "duration", "per-request timeout, such as 30s or 2m", true},
	{"eval.out", "out", "PATH", "", "string", "transcript file to create (default: a new file in the data folder's evals directory)", true},
	{"eval.base-url", "base-url", "URL", "", "string", "one-off OpenAI-compatible server; --model is then the bare model id", true},
	{"eval.api-key-env", "api-key-env", "NAME", "", "string", "environment variable holding the API key", true},

	{"provider.base-url", "base-url", "URL", "", "string", "server address, ending in /v1 for OpenAI-compatible servers (add)", false},
	{"provider.api-key-env", "api-key-env", "NAME", "", "string", "environment variable holding its API key; never the key itself (add)", false},
	{"provider.kind", "kind", "KIND", "", "string", "chat or systemone; leave unset to detect per model (add)", false},

	{"all", "all", "", "false", "bool", "reset every saved setting (reset)", false},
	{"home", "home", "DIR", "", "string", "keep player data and settings in DIR (default: $WORDLE_SOLVER_HOME or the user config folder)", false},
}

func optionByID(id string) *option {
	for i := range allOptions {
		if allOptions[i].id == id {
			return &allOptions[i]
		}
	}
	return nil
}

type command struct {
	name, group, args, summary, help string
	options                          []string
	examples                         []string
	run                              func(*cmdCtx) error
}

var solverOptions = []string{"max", "target", "hard", "sloppy", "copy", "seed"}

var commands = []*command{
	{name: "auto", group: "Play", args: "[--date YYYY-MM-DD | --id N | --from N [--to M]]",
		summary: "Solve a published puzzle by itself (the default command)",
		help: "Looks up the day's answer from the publisher's public daily-puzzle endpoint, then plays a\n" +
			"normal game against it using only the colours each guess earns. The answer is used only to\n" +
			"score guesses. Games are saved, so rerunning a day shows the recorded game.",
		options:  append([]string{"date", "id", "from", "to", "fresh", "dry"}, solverOptions...),
		examples: []string{"wordle-solver", "wordle-solver auto --date 2026-11-12", "wordle-solver auto --from 1900 --to 1910"},
		run:      runAuto},
	{name: "play", group: "Play", summary: "Suggest guesses while you play",
		help: "Suggests a guess each turn. Type the colours the game showed as g/y/b (or 2/1/0), or\n" +
			"'word pattern' to record a different word, for example 'crane bygbb'. With --model, a\n" +
			"model suggests each guess instead; when its reply is unusable, the built-in suggestion\n" +
			"is shown and labelled.",
		options:  append([]string{"play.model", "dry"}, solverOptions...),
		examples: []string{"wordle-solver play", "wordle-solver play --hard", "wordle-solver play --model omlx/clef-flash-4bit"},
		run:      runPlay},
	{name: "sim", group: "Play", args: "WORD", summary: "Watch the solver play a known answer",
		options:  solverOptions,
		examples: []string{"wordle-solver sim crazy", "wordle-solver sim crazy --hard --seed 1"},
		run:      runSim},
	{name: "stats", group: "Play", summary: "Show statistics for your saved games", run: runStats},
	{name: "bench", group: "Analyse", summary: "Benchmark the player model, or compare guess strategies",
		help:     "Without --strategy, plays every answer with the human-like player model. With --strategy,\nplays every answer with that strategy; 'all' runs each in normal and Hard Mode.",
		options:  append([]string{"strategy", "n"}, solverOptions...),
		examples: []string{"wordle-solver bench --strategy all", "wordle-solver bench --strategy entropy --hard", "wordle-solver bench -n 200 --seed 1"},
		run:      runBench},
	{name: "eval", group: "Models", args: "--model PROVIDER/MODEL", summary: "Score a model's games against the entropy strategy",
		help: "Plays games with a language model (chat) or a decision model (systemone) and scores them\n" +
			"against the entropy strategy on the same answers. Chat models reply with a word; decision\n" +
			"models choose from the offered options. Each run writes a JSON lines transcript.",
		options: []string{"eval.model", "eval.games", "eval.set", "eval.seed", "hard", "eval.kind", "eval.options",
			"eval.max-options", "eval.distractors", "eval.temperature", "eval.max-tokens", "eval.timeout", "eval.out",
			"eval.base-url", "eval.api-key-env"},
		examples: []string{
			"wordle-solver eval --model ollama/llama3.2 --games 20 --seed 42",
			"wordle-solver eval --model omlx/clef-flash-4bit --set heldout",
			"wordle-solver eval --model openai/gpt-6.1-terra --options candidates --hard",
			"wordle-solver eval --base-url http://box:8000/v1 --model my-model"},
		run: evalCommand},
	{name: "providers", group: "Models", args: "[list | add NAME --base-url URL | remove NAME | check NAME]",
		summary: "List and configure model servers",
		help: "  list     show built-in and saved providers (the default)\n" +
			"  add      save a provider, or override a built-in's address\n" +
			"  remove   delete a saved provider; a removed override restores the built-in\n" +
			"  check    say whether a provider is reachable and how many models it serves\n\n" +
			"Saved providers keep the name of the key's environment variable, never the key.",
		options: []string{"provider.base-url", "provider.api-key-env", "provider.kind"},
		examples: []string{"wordle-solver providers", "wordle-solver providers check omlx",
			"wordle-solver providers add box --base-url http://box:8000/v1",
			"wordle-solver providers add groq --base-url https://api.groq.com/openai/v1 --api-key-env GROQ_API_KEY"},
		run: providersCommand},
	{name: "models", group: "Models", args: "PROVIDER", summary: "List the models a provider serves",
		examples: []string{"wordle-solver models ollama"}, run: runModels},
	{name: "settings", group: "Setup", args: "[list | get KEY | set KEY VALUE | reset KEY | reset --all]",
		summary: "Show and change saved defaults",
		help: "Each setting comes from, in order: its command-line option, the WORDLE_SOLVER_<KEY>\n" +
			"environment variable (dots and dashes become underscores), the saved config, then its\n" +
			"default. 'list' shows each value and where it came from.",
		options:  []string{"all"},
		examples: []string{"wordle-solver settings", "wordle-solver settings set eval.model omlx/clef-flash-4bit", "wordle-solver settings reset --all"},
		run:      settingsCommand},
	{name: "sync", group: "Setup", summary: "Download every past answer (once; a few minutes)", run: runSync},
	{name: "clean", group: "Setup", summary: "Delete your profile, games and cached answers", options: []string{"y"}, run: runClean},
	{name: "update", group: "Setup", summary: "Install the latest release", options: []string{"force"}, run: runUpdate},
	{name: "version", group: "Setup", summary: "Print the version", run: runVersion},
	{name: "help", group: "Setup", args: "[COMMAND]", summary: "Show help for wordle-solver or a command"},
}

var groups = []string{"Play", "Analyse", "Models", "Setup"}

// help reads the command table, so it is attached after initialisation.
func init() { findCommand("help").run = runHelp }

func findCommand(name string) *command {
	for _, c := range commands {
		if c.name == name {
			return c
		}
	}
	return nil
}

// usageError marks a mistake in how the command was called (exit status 2).
type usageError string

func (e usageError) Error() string { return string(e) }

type cmdCtx struct {
	cmd     *command
	fs      *flag.FlagSet
	args    []string
	config  configFile
	sources map[string]string
}

func (c *cmdCtx) get(id string) string {
	f := c.fs.Lookup(optionByID(id).flag)
	if f == nil {
		return ""
	}
	return f.Value.String()
}
func (c *cmdCtx) int(id string) int           { n, _ := strconv.Atoi(c.get(id)); return n }
func (c *cmdCtx) uint(id string) uint64       { n, _ := strconv.ParseUint(c.get(id), 10, 64); return n }
func (c *cmdCtx) float(id string) float64     { n, _ := strconv.ParseFloat(c.get(id), 64); return n }
func (c *cmdCtx) bool(id string) bool         { b, _ := strconv.ParseBool(c.get(id)); return b }
func (c *cmdCtx) dur(id string) time.Duration { d, _ := time.ParseDuration(c.get(id)); return d }

// set reports whether the option was given on the command line.
func (c *cmdCtx) set(id string) bool {
	found := false
	c.fs.Visit(func(f *flag.Flag) { found = found || f.Name == optionByID(id).flag })
	return found
}

func optionsFor(ids []string) []*option {
	out := []*option{}
	for _, id := range ids {
		out = append(out, optionByID(id))
	}
	return append(out, optionByID("home"))
}

func newFlagSet(name string, opts []*option) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	for _, o := range opts {
		switch o.typ {
		case "bool":
			b, _ := strconv.ParseBool(o.def)
			fs.Bool(o.flag, b, o.usage)
		case "int":
			n, _ := strconv.Atoi(o.def)
			fs.Int(o.flag, n, o.usage)
		case "uint":
			n, _ := strconv.ParseUint(o.def, 10, 64)
			fs.Uint64(o.flag, n, o.usage)
		case "float":
			n, _ := strconv.ParseFloat(o.def, 64)
			fs.Float64(o.flag, n, o.usage)
		case "duration":
			d, _ := time.ParseDuration(o.def)
			fs.Duration(o.flag, d, o.usage)
		default:
			fs.String(o.flag, o.def, o.usage)
		}
	}
	return fs
}

// parseArgs lets options come before or after arguments; "--" ends options.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var rest []string
	for i, a := range args {
		if a == "--" {
			args, rest = args[:i], args[i+1:]
			break
		}
	}
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
	return append(positional, rest...), nil
}

func runCLI(args []string, out, errOut io.Writer) int {
	stdout, stderr = out, errOut
	name := "auto"
	switch {
	case len(args) == 0:
	case args[0] == "-h" || args[0] == "-help" || args[0] == "--help":
		printHelp(out)
		return 0
	case args[0] == "-version" || args[0] == "--version":
		name, args = "version", args[1:]
	case strings.HasPrefix(args[0], "-"):
	default:
		name, args = args[0], args[1:]
	}
	cmd := findCommand(name)
	if cmd == nil {
		msg := fmt.Sprintf("wordle-solver: unknown command %q", name)
		if s := closest(name); s != "" {
			msg += fmt.Sprintf("; did you mean %q?", s)
		}
		fmt.Fprintf(errOut, "%s\nRun 'wordle-solver help' for the list of commands.\n", msg)
		return 2
	}
	opts := optionsFor(cmd.options)
	fs := newFlagSet(cmd.name, opts)
	positional, err := parseArgs(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		printCommandHelp(out, cmd)
		return 0
	}
	if err != nil {
		fmt.Fprintf(errOut, "wordle-solver %s: %v\nRun 'wordle-solver help %s' for its options.\n", cmd.name, err, cmd.name)
		return 2
	}
	if home := fs.Lookup("home").Value.String(); home != "" {
		os.Setenv("WORDLE_SOLVER_HOME", home)
	}
	ctx := &cmdCtx{cmd: cmd, fs: fs, args: positional}
	if ctx.config, err = loadConfig(); err != nil {
		fmt.Fprintf(errOut, "wordle-solver: %v\n", err)
		return 1
	}
	if ctx.sources, err = applySettings(fs, opts, ctx.config); err != nil {
		var u usageError
		if errors.As(err, &u) {
			fmt.Fprintf(errOut, "wordle-solver %s: %v\nRun 'wordle-solver help %s' for its options.\n", cmd.name, err, cmd.name)
			return 2
		}
		fmt.Fprintf(errOut, "wordle-solver: %v\n", err)
		return 1
	}
	if err = cmd.run(ctx); err != nil {
		var u usageError
		if errors.As(err, &u) {
			fmt.Fprintf(errOut, "wordle-solver %s: %v\nRun 'wordle-solver help %s' for its options.\n", cmd.name, err, cmd.name)
			return 2
		}
		fmt.Fprintf(errOut, "wordle-solver %s: %v\n", cmd.name, err)
		return 1
	}
	return 0
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, "wordle-solver solves the five-letter word game, compares guess strategies and scores\nlanguage and decision models on the same games.\n\n")
	fmt.Fprint(w, "Usage:\n  wordle-solver [COMMAND] [OPTIONS] [ARGUMENTS]\n")
	width := 0
	for _, c := range commands {
		width = max(width, len(c.name))
	}
	for _, g := range groups {
		fmt.Fprintf(w, "\n%s:\n", g)
		for _, c := range commands {
			if c.group == g {
				fmt.Fprintf(w, "  %-*s   %s\n", width, c.name, c.summary)
			}
		}
	}
	fmt.Fprint(w, "\nOptions take one or two dashes and may come before or after arguments.\n"+
		"Run 'wordle-solver help COMMAND' (or 'wordle-solver COMMAND --help') for a command's options.\n")
}

func printCommandHelp(w io.Writer, c *command) {
	usage := "wordle-solver " + c.name
	if c.args != "" {
		usage += " " + c.args
	}
	if len(c.options) > 0 {
		usage += " [OPTIONS]"
	}
	fmt.Fprintf(w, "Usage:\n  %s\n\n%s.\n", usage, c.summary)
	if c.help != "" {
		fmt.Fprintf(w, "\n%s\n", c.help)
	}
	fmt.Fprint(w, "\nOptions:\n")
	rows := []row{}
	var keys []string
	for _, o := range optionsFor(c.options) {
		name := "--" + o.flag
		if o.arg != "" {
			name += " " + o.arg
		}
		desc := o.usage
		if o.def != "" && o.def != "false" && o.def != "0" && o.def != "-1" && !strings.Contains(o.usage, "(default") {
			desc += " (default " + o.def + ")"
		}
		if o.setting {
			keys = append(keys, o.id)
		}
		rows = append(rows, row{name, desc})
	}
	rows = append(rows, row{"-h, --help", "show this help"})
	printRows(w, rows)
	if len(keys) > 0 {
		fmt.Fprintln(w)
		for _, line := range wrap("Save defaults with 'wordle-solver settings set KEY VALUE'. Keys here: "+strings.Join(keys, ", ")+".", helpWidth) {
			fmt.Fprintln(w, line)
		}
	}
	switch c.name {
	case "bench":
		fmt.Fprint(w, "\nStrategies:\n")
		rows = rows[:0]
		for _, st := range wordle.Strategies {
			rows = append(rows, row{st.Name, st.Description})
		}
		printRows(w, rows)
	case "settings":
		fmt.Fprint(w, "\nSettings:\n")
		rows = rows[:0]
		for _, o := range settingOptions() {
			rows = append(rows, row{o.id, o.usage})
		}
		printRows(w, rows)
	}
	if len(c.examples) > 0 {
		fmt.Fprint(w, "\nExamples:\n")
		for _, e := range c.examples {
			fmt.Fprintf(w, "  %s\n", e)
		}
	}
}

const helpWidth = 96

type row struct{ name, desc string }

// printRows prints an indented two-column list, wrapping descriptions.
func printRows(w io.Writer, rows []row) {
	width := 0
	for _, r := range rows {
		width = max(width, len(r.name))
	}
	for _, r := range rows {
		for i, line := range wrap(r.desc, helpWidth-width-5) {
			name := ""
			if i == 0 {
				name = r.name
			}
			fmt.Fprintf(w, "  %-*s   %s\n", width, name, line)
		}
	}
}

// wrap splits text into lines of at most width characters at spaces.
func wrap(text string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if line != "" && len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	return append(lines, line)
}

func runHelp(c *cmdCtx) error {
	switch len(c.args) {
	case 0:
		printHelp(stdout)
		return nil
	case 1:
		cmd := findCommand(c.args[0])
		if cmd == nil {
			msg := fmt.Sprintf("unknown command %q", c.args[0])
			if s := closest(c.args[0]); s != "" {
				msg += fmt.Sprintf("; did you mean %q?", s)
			}
			return usageError(msg)
		}
		printCommandHelp(stdout, cmd)
		return nil
	}
	return usageError("help takes at most one command")
}

// closest suggests a command within two edits of a mistyped name.
func closest(name string) string {
	best, bestDist := "", 3
	for _, c := range commands {
		if d := editDistance(name, c.name); d < bestDist {
			best, bestDist = c.name, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func settingOptions() []*option {
	out := []*option{}
	for i := range allOptions {
		if allOptions[i].setting {
			out = append(out, &allOptions[i])
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}
