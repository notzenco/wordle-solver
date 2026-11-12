// Command wordle-solver suggests Wordle guesses that read like a person's game.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"wordle-solver/wordle"
)

const usage = `usage:
  wordle-solver [auto] [-date YYYY-MM-DD | -id N] [-max N] [-target N] [-seed N]
                                                        solve a day's NYT puzzle by itself
  wordle-solver play [-max N] [-target N] [-seed N]     solve a puzzle with you
  wordle-solver sim [-max N] [-target N] [-seed N] WORD watch it solve a known answer
  wordle-solver bench [-max N] [-n N] [-seed N]         results over the answer list
`

func main() {
	args := os.Args[1:]
	cmd := "auto"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	target := fs.Int("target", 0, "aim to finish on this turn (2-6); random if unset")
	maxGuesses := fs.Int("max", 6, "always solve within this many guesses (3-6)")
	sloppy := fs.Float64("sloppy", 0.2, "chance per guess (with turns to spare) of a human slip, 0-1")
	seed := fs.Uint64("seed", 0, "make choices reproducible")
	n := fs.Int("n", 0, "bench: sample this many answers")
	date := fs.String("date", "", "auto: puzzle date as YYYY-MM-DD; today (local time) if unset")
	id := fs.Int("id", -1, "auto: puzzle number, as in \"Wordle 1,973\"")
	fs.Parse(args)

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	if *seed != 0 {
		rng = rand.New(rand.NewPCG(*seed, *seed))
	}
	if *target != 0 && (*target < 2 || *target > 6) {
		fail("-target must be between 2 and 6")
	}
	if *maxGuesses < 3 || *maxGuesses > 6 {
		fail("-max must be between 3 and 6")
	}
	if *sloppy < 0 || *sloppy > 1 {
		fail("-sloppy must be between 0 and 1")
	}
	wordle.DefaultSloppiness = *sloppy

	switch cmd {
	case "auto":
		day := time.Now()
		if *date != "" && *id >= 0 {
			fail("use -date or -id, not both")
		}
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
		auto(rng, day, *target, *maxGuesses)
	case "play":
		play(rng, *target, *maxGuesses)
	case "sim":
		if fs.NArg() != 1 || len(fs.Arg(0)) != 5 {
			fail("sim needs a five-letter answer")
		}
		history := wordle.Play(strings.ToLower(fs.Arg(0)), rng, *target, *maxGuesses)
		for _, t := range history {
			fmt.Printf("%s  %s\n", wordle.Emoji(t.Pattern), strings.ToUpper(t.Guess))
		}
		fmt.Println()
		printShare("", history)
	case "bench":
		bench(rng, *n, *maxGuesses)
	default:
		fs.Usage()
		os.Exit(2)
	}
}

func play(rng *rand.Rand, target, maxGuesses int) {
	s := wordle.NewSolver(rng, target, maxGuesses)
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
	printShare("", s.History)
}

func auto(rng *rand.Rand, day time.Time, target, maxGuesses int) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	p, err := wordle.FetchPuzzle(ctx, wordle.NYTBaseURL, day)
	if err != nil {
		fail(err.Error())
	}
	history := wordle.Play(p.Solution, rng, target, maxGuesses)
	fmt.Printf("Wordle #%s for %s\n\n", thousands(p.Number), p.Date)
	for _, t := range history {
		fmt.Printf("%s  %s\n", wordle.Emoji(t.Pattern), strings.ToUpper(t.Guess))
	}
	fmt.Println()
	printShare(thousands(p.Number)+" ", history)
}

// thousands formats n with commas, as the share text does ("1,973").
func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
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

func printShare(number string, history []wordle.Turn) {
	score := "X"
	if len(history) > 0 && history[len(history)-1].Pattern == wordle.Solved {
		score = fmt.Sprint(len(history))
	}
	fmt.Printf("Wordle %s%s/6\n\n", number, score)
	for _, t := range history {
		fmt.Println(wordle.Emoji(t.Pattern))
	}
}

func bench(rng *rand.Rand, n, maxGuesses int) {
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
		h := wordle.Play(a, rng, 0, maxGuesses)
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
