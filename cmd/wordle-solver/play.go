package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/notzenco/wordle-solver/internal/player"
	"github.com/notzenco/wordle-solver/internal/wordle"
)

func input() *bufio.Scanner {
	if stdin != nil {
		return bufio.NewScanner(stdin)
	}
	return bufio.NewScanner(os.Stdin)
}

// validWords is every word the game accepts as a guess.
func validWords() map[string]bool {
	m := map[string]bool{}
	for _, w := range wordle.ValidWords() {
		m[w] = true
	}
	return m
}

// checkGuess explains why a typed guess can't be played, if it can't.
func checkGuess(word string, valid map[string]bool, history []wordle.Turn, hard bool) error {
	switch {
	case len(word) != 5 || strings.Trim(word, "abcdefghijklmnopqrstuvwxyz") != "":
		return fmt.Errorf("type a five-letter word")
	case !valid[word]:
		return fmt.Errorf("%s isn't in the word list", strings.ToUpper(word))
	case hard && !wordle.HardValid(word, history):
		return fmt.Errorf("Hard Mode: %s must reuse every revealed letter", strings.ToUpper(word))
	}
	return nil
}

// playYourself lets a person play the puzzle with coloured tiles and no
// hints. Published puzzles are saved to the person's own history.
func playYourself(p puzzle, hard, save bool) error {
	dir, mine, err := ownGames()
	if err != nil {
		return err
	}
	if g, ok := mine[p.number]; ok && p.number >= 0 && save {
		fmt.Fprintf(stdout, "%s\nYou already played this one:\n\n", p.title())
		for _, t := range g.Turns {
			fmt.Fprintln(stdout, tiles(t.Guess, t.Pattern))
		}
		return nil
	}
	fmt.Fprintf(stdout, "%s. Six guesses; type a word and press Enter.\n\n", p.title())
	in, valid := input(), validWords()
	var turns []wordle.Turn
	for len(turns) < 6 {
		fmt.Fprintf(stdout, "guess %d> ", len(turns)+1)
		if !in.Scan() {
			fmt.Fprintln(stdout)
			return nil
		}
		word := strings.ToLower(strings.TrimSpace(in.Text()))
		if err := checkGuess(word, valid, turns, hard); err != nil {
			fmt.Fprintln(stdout, "  "+err.Error())
			continue
		}
		t := wordle.Turn{Guess: word, Pattern: wordle.Score(word, p.answer)}
		turns = append(turns, t)
		fmt.Fprintln(stdout, "  "+tiles(t.Guess, t.Pattern))
		if t.Pattern == wordle.Solved {
			break
		}
	}
	fmt.Fprintln(stdout)
	if turns[len(turns)-1].Pattern == wordle.Solved {
		fmt.Fprintf(stdout, "Solved in %d.\n\n", len(turns))
	} else {
		fmt.Fprintf(stdout, "The answer was %s.\n\n", strings.ToUpper(p.answer))
	}
	number := ""
	if p.number >= 0 {
		number = thousands(p.number) + " "
	}
	fmt.Fprint(stdout, shareText(number, turns))
	if !save || p.number < 0 {
		return nil
	}
	mine[p.number] = player.Game{Date: p.date, Turns: turns}
	return player.SaveMine(dir, mine)
}

// assist helps a person play: they type each word they played, it shows the
// colours, then suggests the next word with the chosen approach. With
// manual, the person types the colours too and no answer is needed.
func assist(a advisor, p *puzzle, hard bool) {
	in, valid := input(), validWords()
	if p != nil {
		fmt.Fprintf(stdout, "%s. Type the word you played; the colours are filled in.\n", p.title())
	} else {
		fmt.Fprintln(stdout, "Type the word you played, then the colours the game showed as g/y/b (e.g. bygbb).")
	}
	fmt.Fprintln(stdout, "Press Enter on an empty line to take the suggestion.")
	fmt.Fprintln(stdout)
	var history []wordle.Turn
	suggestion := ""
	for len(history) < 6 {
		fmt.Fprintf(stdout, "word %d> ", len(history)+1)
		if !in.Scan() {
			fmt.Fprintln(stdout)
			return
		}
		word := strings.ToLower(strings.TrimSpace(in.Text()))
		if word == "" {
			word = suggestion
		}
		if err := checkGuess(word, valid, history, hard); err != nil {
			fmt.Fprintln(stdout, "  "+err.Error())
			continue
		}
		var pattern string
		if p != nil {
			pattern = wordle.Score(word, p.answer)
		} else {
			for {
				fmt.Fprint(stdout, "colours> ")
				if !in.Scan() {
					fmt.Fprintln(stdout)
					return
				}
				var err error
				if pattern, err = wordle.ParsePattern(strings.TrimSpace(in.Text())); err == nil {
					break
				}
				fmt.Fprintln(stdout, "  "+err.Error())
			}
		}
		history = append(history, wordle.Turn{Guess: word, Pattern: pattern})
		fmt.Fprintln(stdout, "  "+tiles(word, pattern))
		if pattern == wordle.Solved {
			fmt.Fprintf(stdout, "\nSolved in %d.\n", len(history))
			return
		}
		if len(history) == 6 {
			break
		}
		g, note, err := a.suggest(history, hard)
		if err != nil {
			suggestion = ""
			fmt.Fprintf(stdout, "  no suggestion: %v\n", err)
			continue
		}
		suggestion = g
		fmt.Fprintf(stdout, "  try %s  (%s)\n", strings.ToUpper(g), note)
	}
	if p != nil {
		fmt.Fprintf(stdout, "\nThe answer was %s.\n", strings.ToUpper(p.answer))
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
