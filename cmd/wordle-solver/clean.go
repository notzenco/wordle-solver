package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/notzenco/wordle-solver/internal/player"
)

// clean deletes the player's profile, history and answer cache after
// asking, unless yes is set.
func clean(yes bool) {
	dir, err := player.Dir()
	if err != nil {
		fail(err.Error())
	}
	if !yes {
		fmt.Fprintf(stdout, "This deletes your player profile, game history and cached answers in\n  %s\nContinue? [y/N] ", dir)
		in := bufio.NewScanner(os.Stdin)
		if !in.Scan() || !strings.EqualFold(strings.TrimSpace(in.Text()), "y") {
			fmt.Fprintln(stdout, "Nothing deleted.")
			return
		}
	}
	removed, err := player.Clean(dir)
	if err != nil {
		fail(err.Error())
	}
	if len(removed) == 0 {
		fmt.Fprintln(stdout, "No stored data to delete.")
	} else {
		fmt.Fprintf(stdout, "Deleted %d files. The next game starts a new player.\n", len(removed))
	}
}
