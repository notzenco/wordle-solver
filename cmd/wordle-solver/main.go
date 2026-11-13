// Command wordle-solver plays Wordle in a way that reads like a person's game.
package main

import (
	"flag"
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

const usage = `usage:
  wordle-solver [auto] [-date YYYY-MM-DD | -id N] [-fresh] [-dry]   solve a day's published puzzle by itself
  wordle-solver auto -from N [-to M] [-fresh]                       solve a range of puzzles
  wordle-solver clean [-y]                                          delete your profile, history and cached answers
  wordle-solver update [-force]                                     install the latest release
  wordle-solver settings [set KEY VALUE | reset KEY]                configure defaults
  wordle-solver version                                             print the version
  wordle-solver stats                                               statistics for your saved games
  wordle-solver sync                                                download every past answer
  wordle-solver play                                                solve a puzzle with you
  wordle-solver sim WORD                                            watch it solve a known answer
  wordle-solver bench [-n N]                                        results over the answer list
  wordle-solver bench -strategy NAME|all [-hard]                    compare guess strategies

options for every command:
  -max N      plan to solve within N guesses (3-6, default 6)
  -target N   aim to finish on turn N (2-6)
  -hard       only make guesses Hard Mode allows
  -sloppy P   chance of a human slip per guess with turns to spare (default 0.2)
  -copy       copy the share grid to the clipboard
  -seed N     make choices reproducible
  -home DIR   keep player data in DIR (default $WORDLE_SOLVER_HOME or the user config folder)
`

type options struct {
	target, max int
	sloppy      float64
	hard, copy  bool
}

func main() {
	args := os.Args[1:]
	cmd := "auto"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fmt.Fprintln(os.Stderr, "\nstrategies for bench -strategy (all runs each in normal and Hard Mode):")
		for _, st := range wordle.Strategies {
			fmt.Fprintf(os.Stderr, "  %-10s %s\n", st.Name, st.Description)
		}
	}
	var opts options
	fs.IntVar(&opts.target, "target", 0, "")
	fs.IntVar(&opts.max, "max", 6, "")
	fs.Float64Var(&opts.sloppy, "sloppy", 0.2, "")
	fs.BoolVar(&opts.hard, "hard", false, "")
	fs.BoolVar(&opts.copy, "copy", false, "")
	seed := fs.Uint64("seed", 0, "")
	n := fs.Int("n", 0, "")
	strategy := fs.String("strategy", "", "")
	date := fs.String("date", "", "")
	id := fs.Int("id", -1, "")
	from := fs.Int("from", -1, "")
	to := fs.Int("to", -1, "")
	fresh := fs.Bool("fresh", false, "")
	dry := fs.Bool("dry", false, "")
	yes := fs.Bool("y", false, "")
	force := fs.Bool("force", false, "")
	home := fs.String("home", "", "")
	showVersion := fs.Bool("version", false, "")
	fs.String("model", "", "")
	fs.Int("games", 10, "")
	fs.String("set", "answers", "")
	fs.Float64("temperature", 0, "")
	fs.Int("max-tokens", 64, "")
	fs.Duration("timeout", 60*time.Second, "")
	fs.String("out", "eval.jsonl", "")
	fs.String("base-url", "", "")
	fs.String("api-key-env", "", "")
	fs.Parse(args)

	if runtime.GOOS == "windows" {
		selfupdate.RemoveOld(executable())
	}
	if *showVersion {
		cmd = "version"
	}
	if *home != "" {
		os.Setenv("WORDLE_SOLVER_HOME", *home)
	}

	config, err := loadConfig()
	if err != nil {
		fail(err.Error())
	}
	sources, err := applySettings(fs, config)
	if err != nil {
		fail(err.Error())
	}
	if cmd == "settings" {
		if err := settingsCommand(fs.Args(), fs, config, sources); err != nil {
			fail(err.Error())
		}
		return
	}

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	if *seed != 0 {
		rng = rand.New(rand.NewPCG(*seed, *seed))
	}
	if opts.target != 0 && (opts.target < 2 || opts.target > 6) {
		fail("-target must be between 2 and 6")
	}
	if opts.max < 3 || opts.max > 6 {
		fail("-max must be between 3 and 6")
	}
	if opts.sloppy < 0 || opts.sloppy > 1 {
		fail("-sloppy must be between 0 and 1")
	}

	switch cmd {
	case "auto":
		if *date != "" && *id >= 0 {
			fail("use -date or -id, not both")
		}
		if *from >= 0 || *to >= 0 {
			if *date != "" || *id >= 0 || *dry {
				fail("-from/-to can't be combined with -date, -id or -dry")
			}
			last := *to
			if last < 0 {
				last = wordle.NumberForDate(time.Now())
			}
			if *from < 0 || *from > last {
				fail("-from must be set and not after -to")
			}
			var numbers []int
			for n := *from; n <= last; n++ {
				numbers = append(numbers, n)
			}
			autoRange(rng, opts, loadStore(rng), numbers, *fresh)
			return
		}
		day := time.Now()
		if *id >= 0 {
			day = wordle.DateForNumber(*id)
		}
		if *date != "" {
			d, err := time.ParseInLocation(time.DateOnly, *date, time.Local)
			if err != nil {
				fail("-date must look like 2026-11-13")
			}
			day = d
		}
		var store *player.Store
		if !*dry {
			store = loadStore(rng)
		}
		auto(rng, opts, store, day, *fresh)
	case "clean":
		clean(*yes)
	case "update":
		update(*force)
	case "version":
		fmt.Println("wordle-solver", versionString())
	case "stats":
		printStats(loadStore(rng))
	case "sync":
		syncAnswers(loadStore(rng))
	case "play":
		var store *player.Store
		if !*dry {
			store = loadStore(rng)
		}
		play(newSolver(rng, opts, store), opts)
	case "sim":
		if fs.NArg() != 1 || len(fs.Arg(0)) != 5 {
			fail("sim needs a five-letter answer")
		}
		turns := newSolver(rng, opts, nil).Solve(strings.ToLower(fs.Arg(0)))
		printGame("", turns, opts.copy)
	case "bench":
		if *strategy != "" {
			benchStrategies(*strategy, opts.hard)
			return
		}
		bench(rng, opts, *n)
	default:
		fs.Usage()
		os.Exit(2)
	}
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

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
