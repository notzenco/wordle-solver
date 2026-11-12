// Package wordle scores guesses and picks human-looking next moves.
package wordle

import (
	_ "embed"
	"fmt"
	"strings"
)

const (
	Green  = 'g'
	Yellow = 'y'
	Grey   = 'b'
	Solved = "ggggg"
)

//go:embed data/answers.txt
var answersTxt string

// Answers is the list of common five-letter words Wordle draws answers from.
var Answers = strings.Fields(answersTxt)

// Score returns the feedback pattern for guess against answer, handling
// repeated letters the way Wordle does.
func Score(guess, answer string) string {
	var result [5]byte
	var unmatched [26]int
	for i := range 5 {
		if guess[i] == answer[i] {
			result[i] = Green
		} else {
			result[i] = Grey
			unmatched[answer[i]-'a']++
		}
	}
	for i := range 5 {
		if result[i] != Green && unmatched[guess[i]-'a'] > 0 {
			result[i] = Yellow
			unmatched[guess[i]-'a']--
		}
	}
	return string(result[:])
}

type Turn struct {
	Guess, Pattern string
}

// Filter keeps the words consistent with every turn so far.
func Filter(words []string, history []Turn) []string {
	var out []string
outer:
	for _, w := range words {
		for _, t := range history {
			if Score(t.Guess, w) != t.Pattern {
				continue outer
			}
		}
		out = append(out, w)
	}
	return out
}

// ParsePattern accepts g/y/b letters, 2/1/0 digits or the coloured squares.
func ParsePattern(text string) (string, error) {
	var out []byte
	for _, r := range strings.ToLower(text) {
		switch r {
		case ' ', '\t':
		case 'g', '2', '🟩':
			out = append(out, Green)
		case 'y', '1', '🟨':
			out = append(out, Yellow)
		case 'b', 'x', '.', '-', '0', '⬛', '⬜':
			out = append(out, Grey)
		default:
			return "", fmt.Errorf("unexpected %q in pattern", r)
		}
	}
	if len(out) != 5 {
		return "", fmt.Errorf("pattern needs 5 marks of g/y/b (or 2/1/0), got %d", len(out))
	}
	return string(out), nil
}

func Emoji(pattern string) string {
	return strings.NewReplacer("g", "🟩", "y", "🟨", "b", "⬛").Replace(pattern)
}
