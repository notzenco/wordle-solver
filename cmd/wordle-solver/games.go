package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/notzenco/wordle-solver/internal/player"
	"github.com/notzenco/wordle-solver/internal/wordle"
)

// newSolver applies the options and, given a store, the player's habits:
// a favourite opener and a target that keeps their record believable.
func newSolver(rng *rand.Rand, opts options, store *player.Store) *wordle.Solver {
	s := wordle.NewSolver(rng, opts.target, opts.max)
	s.Sloppiness = opts.sloppy
	s.Hard = opts.hard
	if store != nil {
		s.AddWords(store.KnownAnswers()...)
		s.Opener = store.Opener(rng)
	}
	return s
}

func auto(rng *rand.Rand, opts options, store *player.Store, day time.Time, fresh bool) {
	number := wordle.NumberForDate(day)
	if store != nil && !fresh {
		if g, ok := store.Game(number); ok {
			fmt.Printf("%s(Already played; -fresh plays it again.)\n\n", autoHeader(number, g.Date))
			printGame(thousands(number)+" ", g.Turns, opts.copy)
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	p, err := wordle.FetchPuzzle(ctx, wordle.NYTBaseURL, day)
	if err != nil {
		fail(err.Error())
	}
	if store != nil {
		store.AddAnswer(p.Number, p.Solution)
	}
	turns := solverFor(rng, opts, store, p.Number).Solve(p.Solution)
	fmt.Printf("%s\n", autoHeader(p.Number, p.Date))
	printGame(thousands(p.Number)+" ", turns, opts.copy)

	if store != nil {
		store.Record(p.Number, p.Date, turns)
		if err := store.Save(); err != nil {
			fail(fmt.Sprintf("saving game: %v", err))
		}
	}
}

// autoHeader introduces a game against a fetched answer and says how that
// answer is used.
func autoHeader(number int, date string) string {
	return fmt.Sprintf("Wordle #%s for %s\n%s\n", thousands(number), date, answerNote)
}

const answerNote = "The answer was looked up from the published daily puzzle and used only to score each guess."

// solverFor is newSolver for puzzle number, also ruling out earlier answers
// for a player in the habit of skipping them.
func solverFor(rng *rand.Rand, opts options, store *player.Store, number int) *wordle.Solver {
	s := newSolver(rng, opts, store)
	if store != nil && store.Profile.SkipPastAnswers {
		s.Exclude(store.AnswersBefore(number)...)
	}
	return s
}

// autoRange solves the numbered puzzles in order, one line each, saving
// after every game so an interrupted run keeps its progress.
func autoRange(rng *rand.Rand, opts options, store *player.Store, numbers []int, fresh bool) {
	if len(numbers) == 0 {
		return
	}
	fmt.Println(answerNote)
	fmt.Println()
	for _, n := range numbers {
		if g, ok := store.Game(n); ok && !fresh {
			fmt.Printf("%s  (already played)\n", summary(n, g.Turns))
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		p, err := wordle.FetchPuzzle(ctx, wordle.NYTBaseURL, wordle.DateForNumber(n))
		cancel()
		if err != nil {
			fail(err.Error())
		}
		store.AddAnswer(p.Number, p.Solution)
		turns := solverFor(rng, opts, store, p.Number).Solve(p.Solution)
		store.Record(p.Number, p.Date, turns)
		if err := store.Save(); err != nil {
			fail(fmt.Sprintf("saving game: %v", err))
		}
		fmt.Println(summary(n, turns))
		time.Sleep(200 * time.Millisecond) // go easy on NYT's server
	}
	fmt.Println()
	printStats(store)
}

// syncAnswers downloads every past answer not yet cached, a few at a time.
func syncAnswers(store *player.Store) {
	missing := store.Missing(wordle.NumberForDate(time.Now()))
	if len(missing) == 0 {
		fmt.Printf("All %d answers already downloaded.\n", len(store.Answers))
		return
	}
	fmt.Printf("Downloading %d answers...\n", len(missing))
	type result struct {
		p   wordle.Puzzle
		err error
	}
	jobs := make(chan int)
	results := make(chan result)
	const workers = 4
	for range workers {
		go func() {
			for n := range jobs {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				p, err := wordle.FetchPuzzle(ctx, wordle.NYTBaseURL, wordle.DateForNumber(n))
				cancel()
				results <- result{p, err}
				time.Sleep(100 * time.Millisecond) // go easy on NYT's server
			}
		}()
	}
	go func() {
		for _, n := range missing {
			jobs <- n
		}
		close(jobs)
	}()
	failed := 0
	for i := range missing {
		r := <-results
		if r.err != nil {
			failed++
			fmt.Fprintln(os.Stderr, r.err)
		} else {
			store.AddAnswer(r.p.Number, r.p.Solution)
		}
		if (i+1)%250 == 0 {
			fmt.Printf("  %d/%d\n", i+1, len(missing))
		}
	}
	if err := store.Save(); err != nil {
		fail(fmt.Sprintf("saving answers: %v", err))
	}
	extra := 0
	for _, w := range store.KnownAnswers() {
		if !slices.Contains(wordle.Answers, w) {
			extra++
		}
	}
	fmt.Printf("Done: %d answers known, %d of them newer than the original list.\n", len(store.Answers), extra)
	if failed > 0 {
		fail(fmt.Sprintf("%d downloads failed; run sync again to retry them", failed))
	}
}

func bench(rng *rand.Rand, opts options, n int) {
	answers := wordle.Answers
	if n > 0 && n < len(answers) {
		answers = make([]string, n)
		for i, j := range rng.Perm(len(wordle.Answers))[:n] {
			answers[i] = wordle.Answers[j]
		}
	}
	var counts [8]int // index 7 counts losses
	solvedTurns := 0
	for _, a := range answers {
		h := newSolver(rng, opts, nil).Solve(a)
		if h[len(h)-1].Pattern == wordle.Solved {
			counts[len(h)]++
			solvedTurns += len(h)
		} else {
			counts[7]++
		}
	}
	total := float64(len(answers))
	for turn := 1; turn <= 7; turn++ {
		label := fmt.Sprint(turn)
		if turn == 7 {
			label = "X"
		}
		c := counts[turn]
		fmt.Printf("%s: %5d  %5.1f%%  %s\n", label, c, 100*float64(c)/total,
			strings.Repeat("#", int(60*float64(c)/total+0.5)))
	}
	fmt.Printf("mean %.2f guesses, %d lost\n",
		float64(solvedTurns)/float64(len(answers)-counts[7]), counts[7])
}

// benchStrategies plays every answer with the named strategy, or with every
// strategy in both modes for "all", and prints one row per run.
func benchStrategies(name string, hard bool) {
	strategies := wordle.Strategies
	modes := []bool{false, true}
	if name != "all" {
		st, ok := wordle.StrategyByName(name)
		if !ok {
			fail(fmt.Sprintf("unknown strategy %q; run 'wordle-solver -h' for the list", name))
		}
		strategies, modes = []wordle.Strategy{st}, []bool{hard}
	}
	fmt.Printf("%-9s %-6s %-6s %5s %6s %6s %6s %6s %6s %6s %7s %8s\n",
		"strategy", "mode", "opener", "mean", "1", "2", "3", "4", "5", "6", "failed", "runtime")
	for _, hard := range modes {
		for _, st := range strategies {
			r := wordle.Benchmark(st, hard, wordle.Answers)
			mode := "normal"
			if hard {
				mode = "hard"
			}
			fmt.Printf("%-9s %-6s %-6s %5.3f", r.Strategy, mode, strings.ToUpper(r.Opener), r.Mean)
			for turn := 1; turn <= 6; turn++ {
				fmt.Printf(" %6d", r.Wins[turn])
			}
			fmt.Printf(" %7d %7.1fs\n", r.Failed, r.Runtime.Seconds())
		}
	}
	fmt.Printf("\n%d answers per run.\n", len(wordle.Answers))
}
