package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/notzenco/wordle-solver/internal/player"
)

// clean deletes the saved data and settings after asking, unless yes is
// set. Eval transcripts are kept, since they may be research records.
func clean(yes bool) {
	dir, err := player.Dir()
	if err != nil {
		fail(err.Error())
	}
	if !yes {
		fmt.Fprintf(stdout, "This deletes the player profile, both game histories, cached answers, settings\nand saved providers in\n  %s\nEval transcripts are kept. Continue? [y/N] ", dir)
		in := input()
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
	evals := filepath.Join(dir, "evals")
	if entries, err := os.ReadDir(evals); err == nil && len(entries) > 0 {
		fmt.Fprintf(stdout, "Kept %d eval transcripts in %s; delete that folder yourself if you no longer need them.\n", len(entries), evals)
	}
}
