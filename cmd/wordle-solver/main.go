// Command wordle-solver solves the five-letter word game, compares guess
// strategies and scores language and decision models on the same games.
package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/notzenco/wordle-solver/internal/player"
	"github.com/notzenco/wordle-solver/internal/selfupdate"
	"github.com/notzenco/wordle-solver/internal/wordle"
)

type options struct {
	target, max int
	sloppy      float64
	hard, copy  bool
}

func main() {
	if runtime.GOOS == "windows" {
		selfupdate.RemoveOld(executable())
	}
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

// solver reads and checks the options shared by the commands that play. A
// zero --seed draws a new one, kept in c.seed so it can be shown.
func (c *cmdCtx) solver() (options, *rand.Rand, error) {
	o := options{target: c.int("target"), max: c.int("max"), sloppy: c.float("sloppy"), hard: c.bool("hard"), copy: c.bool("copy")}
	switch {
	case o.target != 0 && (o.target < 2 || o.target > 6):
		return o, nil, usageError("--target must be between 2 and 6")
	case o.max < 3 || o.max > 6:
		return o, nil, usageError("--max must be between 3 and 6")
	case o.sloppy < 0 || o.sloppy > 1:
		return o, nil, usageError("--sloppy must be between 0 and 1")
	}
	c.seed = c.uint("seed")
	for c.seed == 0 {
		c.seed = rand.Uint64() % 1_000_000_000 // short enough to retype
	}
	return o, rand.New(rand.NewPCG(c.seed, c.seed)), nil
}

func (c *cmdCtx) noArgs() error {
	if len(c.args) > 0 {
		return usageError(fmt.Sprintf("%s takes no arguments (got %q)", c.cmd.name, strings.Join(c.args, " ")))
	}
	return nil
}

func runAuto(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	opts, rng, err := c.solver()
	if err != nil {
		return err
	}
	by, m, err := c.approach()
	if err != nil {
		return err
	}
	from, to := c.int("from"), c.int("to")
	if from >= 0 || to >= 0 {
		if by != "human" {
			return usageError("--from and --to need --by human")
		}
		if c.get("date") != "" || c.int("id") >= 0 || c.bool("dry") || c.bool("random") {
			return usageError("--from and --to can't be combined with --date, --id, --random or --dry")
		}
		last := to
		if last < 0 {
			last = wordle.NumberForDate(time.Now())
		}
		if from < 0 || from > last {
			return usageError("--from must be set and not after --to")
		}
		var numbers []int
		for n := from; n <= last; n++ {
			numbers = append(numbers, n)
		}
		autoRange(rng, opts, loadStore(rng), numbers, c.bool("fresh"))
		return nil
	}
	if by != "human" {
		p, err := c.puzzle(rng)
		if err != nil {
			return err
		}
		autoClues(by, m, p, opts)
		return nil
	}
	if c.bool("random") {
		p, err := c.puzzle(rng)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s\n\n", p.title())
		printGame("", newSolver(rng, opts, nil).Solve(p.answer), opts.copy)
		fmt.Fprintf(stdout, "\nSeed %d; --seed %d replays this game. Practice games are not saved.\n", c.seed, c.seed)
		return nil
	}
	date, id := c.get("date"), c.int("id")
	if date != "" && id >= 0 {
		return usageError("use --date or --id, not both")
	}
	day := time.Now()
	if id >= 0 {
		day = wordle.DateForNumber(id)
	}
	if date != "" {
		d, err := time.ParseInLocation(time.DateOnly, date, time.Local)
		if err != nil {
			return usageError("--date must look like 2026-11-13")
		}
		day = d
	}
	var store *player.Store
	if !c.bool("dry") {
		store = loadStore(rng)
	}
	if auto(rng, opts, store, day, c.bool("fresh")) {
		fmt.Fprintf(stdout, "\nSeed %d; --seed %d --fresh replays this game.\n", c.seed, c.seed)
	}
	return nil
}

func runPlay(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	p, err := c.puzzle(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	if err != nil {
		return err
	}
	return playYourself(p, c.bool("hard"), !c.bool("dry"))
}

func runAssist(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	opts, rng, err := c.solver()
	if err != nil {
		return err
	}
	by, m, err := c.approach()
	if err != nil {
		return err
	}
	var a advisor
	switch by {
	case "human":
		a = &humanAdvisor{newSolver(rng, opts, nil)}
	case "solver":
		a = newSolverAdvisor()
	default:
		a = m
	}
	if c.bool("manual") {
		if c.get("date") != "" || c.int("id") >= 0 || c.bool("random") {
			return usageError("--manual looks nothing up, so it can't be combined with --date, --id or --random")
		}
		assist(a, nil, opts.hard)
		return nil
	}
	p, err := c.puzzle(rng)
	if err != nil {
		return err
	}
	assist(a, &p, opts.hard)
	return nil
}

func runSim(c *cmdCtx) error {
	if len(c.args) != 1 || !wordle.IsWord(strings.ToLower(c.args[0])) {
		return usageError("sim needs one five-letter answer made of letters a-z, such as 'wordle-solver sim crazy'")
	}
	opts, rng, err := c.solver()
	if err != nil {
		return err
	}
	turns := newSolver(rng, opts, nil).Solve(strings.ToLower(c.args[0]))
	printGame("", turns, opts.copy)
	return nil
}

func runBench(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	opts, rng, err := c.solver()
	if err != nil {
		return err
	}
	if name := c.get("strategy"); name != "" {
		if _, ok := wordle.StrategyByName(name); !ok && name != "all" {
			return usageError(fmt.Sprintf("unknown strategy %q; run 'wordle-solver help bench' for the list", name))
		}
		benchStrategies(name, opts.hard)
		return nil
	}
	bench(rng, opts, c.int("n"))
	return nil
}

func runStats(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	today := wordle.NumberForDate(time.Now())
	if c.bool("mine") {
		_, games, err := ownGames()
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Your own games ('wordle-solver play'):")
		fmt.Fprintln(stdout)
		printStats(player.StatsFor(games, today))
		return nil
	}
	fmt.Fprintln(stdout, "The human-like player's games ('wordle-solver auto'); --mine shows yours:")
	fmt.Fprintln(stdout)
	printStats(loadStore(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))).Stats(today))
	return nil
}

func runSync(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	syncAnswers(loadStore(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))))
	return nil
}

func runClean(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	clean(c.bool("y"))
	return nil
}

func runUpdate(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	update(c.bool("force"))
	return nil
}

func runVersion(c *cmdCtx) error {
	if err := c.noArgs(); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "wordle-solver", versionString())
	return nil
}

func runModels(c *cmdCtx) error {
	if len(c.args) != 1 {
		return usageError("models needs one provider, such as 'wordle-solver models ollama'")
	}
	p, err := findProvider(c.config, c.args[0])
	if err != nil {
		return err
	}
	models, err := fetchModels(p)
	if err != nil {
		return err
	}
	for _, m := range models {
		fmt.Fprintln(stdout, m)
	}
	return nil
}

func loadStore(rng *rand.Rand) *player.Store {
	dir, err := player.Dir()
	if err != nil {
		fail(err.Error())
	}
	store, err := player.Load(dir, rng)
	if err != nil {
		fail(fmt.Sprintf("reading player data in %s: %v", dir, err))
	}
	return store
}

// fail ends the program from deep inside a command that cannot recover.
func fail(msg string) {
	fmt.Fprintln(stderr, msg)
	os.Exit(1)
}
