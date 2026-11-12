// Command wordle-solver plays Wordle in a way that reads like a person's game.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"wordle-solver/player"
	"wordle-solver/wordle"
)

const usage = `usage:
  wordle-solver [auto] [-date YYYY-MM-DD | -id N] [-fresh] [-dry]   solve a day's NYT puzzle by itself
  wordle-solver auto -from N [-to M] [-fresh]                       solve a range of puzzles
  wordle-solver auto -catchup                                       solve every day since the last game
  wordle-solver schedule [-at HH:MM] [-remove]                      solve each day automatically (macOS)
  wordle-solver stats                                               your NYT-style statistics
  wordle-solver sync                                                download every past answer
  wordle-solver play                                                solve a puzzle with you
  wordle-solver sim WORD                                            watch it solve a known answer
  wordle-solver bench [-n N]                                        results over the answer list

options for every command:
  -max N      always solve within N guesses (3-6, default 6)
  -target N   aim to finish on turn N (2-6)
  -hard       only make guesses Hard Mode allows
  -sloppy P   chance of a human slip per guess with turns to spare (default 0.2)
  -copy       copy the share grid to the clipboard
  -seed N     make choices reproducible
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
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	var opts options
	fs.IntVar(&opts.target, "target", 0, "")
	fs.IntVar(&opts.max, "max", 6, "")
	fs.Float64Var(&opts.sloppy, "sloppy", 0.2, "")
	fs.BoolVar(&opts.hard, "hard", false, "")
	fs.BoolVar(&opts.copy, "copy", false, "")
	seed := fs.Uint64("seed", 0, "")
	n := fs.Int("n", 0, "")
	date := fs.String("date", "", "")
	id := fs.Int("id", -1, "")
	from := fs.Int("from", -1, "")
	to := fs.Int("to", -1, "")
	fresh := fs.Bool("fresh", false, "")
	catchup := fs.Bool("catchup", false, "")
	at := fs.String("at", "00:05", "")
	remove := fs.Bool("remove", false, "")
	dry := fs.Bool("dry", false, "")
	fs.Parse(args)

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
		if *catchup {
			if *date != "" || *id >= 0 || *dry || *from >= 0 || *to >= 0 {
				fail("-catchup can't be combined with -date, -id, -dry, -from or -to")
			}
			store := loadStore(rng)
			fmt.Printf("== %s\n", time.Now().Format("2006-01-02 15:04"))
			autoRange(rng, opts, store, store.Unplayed(wordle.NumberForDate(time.Now())), false)
			return
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
	case "schedule":
		schedule(*at, *remove)
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

// newSolver applies the options and, given a store, the player's habits:
// a favourite opener and a target that keeps their record believable.
func newSolver(rng *rand.Rand, opts options, store *player.Store) *wordle.Solver {
	s := wordle.NewSolver(rng, opts.target, opts.max)
	s.Sloppiness = opts.sloppy
	s.Hard = opts.hard
	if store != nil {
		s.AddWords(store.KnownAnswers()...)
		s.Opener = store.Opener(rng)
		if opts.target == 0 {
			s.Target = store.Target(rng, opts.max)
		}
	}
	return s
}

func auto(rng *rand.Rand, opts options, store *player.Store, day time.Time, fresh bool) {
	number := wordle.NumberForDate(day)
	if store != nil && !fresh {
		if g, ok := store.Game(number); ok {
			fmt.Printf("Wordle #%s for %s (already played; -fresh plays it again)\n\n", thousands(number), g.Date)
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
	fmt.Printf("Wordle #%s for %s\n\n", thousands(p.Number), p.Date)
	printGame(thousands(p.Number)+" ", turns, opts.copy)

	if store != nil {
		store.Record(p.Number, p.Date, turns)
		if err := store.Save(); err != nil {
			fail(fmt.Sprintf("saving game: %v", err))
		}
	}
}

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
		fmt.Println("Nothing to play: today's puzzle is already done.")
		return
	}
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

// summary is one game on one line: number, score and coloured rows.
func summary(n int, turns []wordle.Turn) string {
	rows := make([]string, len(turns))
	for i, t := range turns {
		rows[i] = wordle.Emoji(t.Pattern)
	}
	return fmt.Sprintf("Wordle %-6s %s  %s", thousands(n), score(turns), strings.Join(rows, " "))
}

func score(turns []wordle.Turn) string {
	if len(turns) > 0 && turns[len(turns)-1].Pattern == wordle.Solved {
		return fmt.Sprintf("%d/6", len(turns))
	}
	return "X/6"
}

func printStats(store *player.Store) {
	st := store.Stats(wordle.NumberForDate(time.Now()))
	if st.Played == 0 {
		fmt.Println("No games played yet.")
		return
	}
	fmt.Printf("Played %d   Win %% %d   Current streak %d   Max streak %d\n\n",
		st.Played, 100*st.Won/st.Played, st.CurrentStreak, st.MaxStreak)
	most := 1
	for _, c := range st.Guesses {
		most = max(most, c)
	}
	fmt.Println("Guess distribution")
	for turn := 1; turn <= 6; turn++ {
		c := st.Guesses[turn]
		fmt.Printf("%d %s %d\n", turn, strings.Repeat("█", 1+30*c/most), c)
	}
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

func play(s *wordle.Solver, opts options) {
	in := bufio.NewScanner(os.Stdin)
	fmt.Println("Type each suggestion into Wordle, then enter the colours it showed")
	fmt.Println("as g/y/b (e.g. bygbb). To play your own word instead: 'word pattern'.")
	fmt.Println()
	for s.Turn() <= 6 {
		suggestion, err := s.NextGuess()
		if err != nil {
			fail(err.Error())
		}
		fmt.Printf("Guess %d: %s   (%d possible answers)\n",
			s.Turn(), strings.ToUpper(suggestion), len(s.Candidates()))
		var guess, pattern string
		for {
			fmt.Print("  result> ")
			if !in.Scan() {
				return
			}
			guess, pattern, err = parseResult(in.Text(), suggestion)
			if err == nil {
				break
			}
			fmt.Println("  " + err.Error())
		}
		s.Record(guess, pattern)
		if pattern == wordle.Solved {
			break
		}
	}
	fmt.Println()
	fmt.Print(shareText("", s.History))
	if opts.copy {
		copyText(shareText("", s.History))
	}
}

// parseResult reads "pattern" for the suggested word or "word pattern" for
// a word the player chose themselves.
func parseResult(line, suggestion string) (guess, pattern string, err error) {
	fields := strings.Fields(strings.ToLower(line))
	guess = suggestion
	if len(fields) > 1 && len(fields[0]) == 5 && strings.Trim(fields[0], "abcdefghijklmnopqrstuvwxyz") == "" {
		if _, perr := wordle.ParsePattern(fields[0]); perr != nil {
			guess, fields = fields[0], fields[1:]
		}
	}
	pattern, err = wordle.ParsePattern(strings.Join(fields, ""))
	return guess, pattern, err
}

// printGame shows the guesses with their colours, then the share text.
func printGame(number string, turns []wordle.Turn, toClipboard bool) {
	for _, t := range turns {
		fmt.Printf("%s  %s\n", wordle.Emoji(t.Pattern), strings.ToUpper(t.Guess))
	}
	fmt.Println()
	share := shareText(number, turns)
	fmt.Print(share)
	if toClipboard {
		copyText(share)
	}
}

func shareText(number string, turns []wordle.Turn) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Wordle %s%s\n\n", number, score(turns))
	for _, t := range turns {
		b.WriteString(wordle.Emoji(t.Pattern) + "\n")
	}
	return b.String()
}

func copyText(text string) {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(strings.TrimRight(text, "\n"))
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\ncould not copy to the clipboard: %v\n", err)
		return
	}
	fmt.Println("\n(copied to the clipboard)")
}

// thousands formats n with commas, as the share text does ("1,973").
func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
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

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
