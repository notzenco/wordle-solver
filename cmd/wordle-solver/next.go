package main

import (
	"fmt"
	"strings"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

// runNext ranks the next guesses for clues typed as WORD:COLOURS, without a
// game: a quick answer for someone mid-puzzle elsewhere.
func runNext(c *cmdCtx) error {
	top := c.int("top")
	if top < 1 {
		return usageError("--top must be at least 1")
	}
	var history []wordle.Turn
	for _, arg := range c.args {
		word, colours, ok := strings.Cut(strings.ToLower(arg), ":")
		if !ok {
			return usageError(fmt.Sprintf("%q needs the colours after a colon, such as slate:bygbb", arg))
		}
		if !wordle.IsValid(word) {
			return usageError(fmt.Sprintf("%s is not in the word list", strings.ToUpper(word)))
		}
		pattern, err := wordle.ParsePattern(colours)
		if err != nil {
			return usageError(fmt.Sprintf("%s: %v", arg, err))
		}
		history = append(history, wordle.Turn{Guess: word, Pattern: pattern})
	}
	if n := len(history); n > 0 && history[n-1].Pattern == wordle.Solved {
		fmt.Fprintf(stdout, "Solved: %s.\n", strings.ToUpper(history[n-1].Guess))
		return nil
	}
	cands, ranked := wordle.Rank(history, c.bool("hard"), top)
	if c.bool("json") {
		return writeJSONOut(map[string]any{"candidates": cands, "guesses": ranked})
	}
	if len(cands) == 0 {
		g, ok := newSolverAdvisor().st.Suggest(history, c.bool("hard"))
		if !ok {
			return fmt.Errorf("no word fits every clue; check the colours")
		}
		fmt.Fprintf(stdout, "No listed answer fits; the most familiar word that does is %s.\n", strings.ToUpper(g))
		return nil
	}
	fmt.Fprintf(stdout, "%d listed answers fit", len(cands))
	if len(cands) <= 20 {
		fmt.Fprintf(stdout, ": %s", strings.ToUpper(strings.Join(cands, " ")))
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "\n  %-6s %5s %9s %6s\n", "guess", "bits", "expected", "worst")
	for _, r := range ranked {
		mark := " "
		if r.Fits {
			mark = "*"
		}
		fmt.Fprintf(stdout, "  %s%-5s %5.2f %9.1f %6d\n", mark, strings.ToUpper(r.Word), r.Bits, r.Expected, r.Worst)
	}
	return nil
}
