package main

import (
	"os"
	"strings"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

// colour reports whether stdout is a terminal that should get coloured tiles.
// NO_COLOR (https://no-color.org) turns them off.
func colour() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := stdout.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// tiles shows a guess with its colours: lettered tiles in a terminal, or the
// letters followed by the emoji grid elsewhere.
func tiles(guess, pattern string) string {
	guess = strings.ToUpper(guess)
	if !colour() {
		return strings.Join(strings.Split(guess, ""), " ") + "   " + wordle.Emoji(pattern)
	}
	var b strings.Builder
	for i := range guess {
		style := "1;37;100" // white on grey
		switch pattern[i] {
		case 'g':
			style = "1;30;42" // black on green
		case 'y':
			style = "1;30;43" // black on yellow
		}
		b.WriteString("\x1b[" + style + "m " + guess[i:i+1] + " \x1b[0m ")
	}
	return strings.TrimRight(b.String(), " ")
}
