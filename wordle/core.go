// Package wordle scores guesses and picks human-looking next moves.
package wordle

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"
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

// frequency.txt holds how often each answer word appears in English film and
// TV subtitles (FrequencyWords, OpenSubtitles 2018, CC BY-SA 4.0).
//
//go:embed data/frequency.txt
var frequencyTxt string

var frequency = func() map[string]float64 {
	f := map[string]float64{}
	fields := strings.Fields(frequencyTxt)
	for i := 0; i+1 < len(fields); i += 2 {
		n, _ := strconv.ParseFloat(fields[i+1], 64)
		f[fields[i]] = n
	}
	return f
}()

// familiarity grows with how everyday a word is: HOUSE scores about eight
// times NADIR. Words without a count score as the rarest.
func familiarity(w string) float64 {
	return math.Pow(math.Log1p(frequency[w])+1, 3)
}

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
	Guess   string `json:"guess"`
	Pattern string `json:"pattern"`
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

// solvedCode is scoreCode for an all-green result.
const solvedCode = 242

// scoreCode is Score as a base-3 number (grey 0, yellow 1, green 2), which
// lets hot loops bucket results in an array instead of a map.
func scoreCode(guess, answer string) int {
	var green [5]bool
	var unmatched [26]int8
	code := 0
	for i := range 5 {
		if guess[i] == answer[i] {
			green[i] = true
		} else {
			unmatched[answer[i]-'a']++
		}
	}
	for i, pow := 0, 1; i < 5; i, pow = i+1, pow*3 {
		switch c := guess[i] - 'a'; {
		case green[i]:
			code += 2 * pow
		case unmatched[c] > 0:
			code += pow
			unmatched[c]--
		}
	}
	return code
}

// HardValid reports whether Hard Mode accepts guess after history: every
// green stays in place and every revealed letter is reused, as many times
// as one earlier guess revealed it. Grey letters are allowed.
func HardValid(guess string, history []Turn) bool {
	for _, t := range history {
		var need, have [26]int
		for i := range 5 {
			if t.Pattern[i] == Green && guess[i] != t.Guess[i] {
				return false
			}
			if t.Pattern[i] != Grey {
				need[t.Guess[i]-'a']++
			}
			have[guess[i]-'a']++
		}
		for c := range 26 {
			if have[c] < need[c] {
				return false
			}
		}
	}
	return true
}
