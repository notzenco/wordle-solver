package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

func play(s *wordle.Solver, opts options) {
	in := bufio.NewScanner(os.Stdin)
	fmt.Fprintln(stdout, "Type each suggestion into Wordle, then enter the colours it showed")
	fmt.Fprintln(stdout, "as g/y/b (e.g. bygbb). To play your own word instead: 'word pattern'.")
	fmt.Fprintln(stdout)
	for s.Turn() <= 6 {
		suggestion, err := s.NextGuess()
		if err != nil {
			fail(err.Error())
		}
		fmt.Fprintf(stdout, "Guess %d: %s   (%d possible answers)\n",
			s.Turn(), strings.ToUpper(suggestion), len(s.Candidates()))
		var guess, pattern string
		for {
			fmt.Fprint(stdout, "  result> ")
			if !in.Scan() {
				return
			}
			guess, pattern, err = parseResult(in.Text(), suggestion)
			if err == nil {
				break
			}
			fmt.Fprintln(stdout, "  "+err.Error())
		}
		s.Record(guess, pattern)
		if pattern == wordle.Solved {
			break
		}
	}
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, shareText("", s.History))
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
