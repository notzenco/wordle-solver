package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/notzenco/wordle-solver/internal/player"
	"github.com/notzenco/wordle-solver/internal/schedule"
)

// clean deletes the player's profile, history, answer cache and the daily
// run's log after asking, unless yes is set.
func clean(yes bool) {
	dir, err := player.Dir()
	if err != nil {
		fail(err.Error())
	}
	log, err := schedule.LogPath()
	if err != nil {
		fail(err.Error())
	}
	if !yes {
		fmt.Printf("This deletes your player profile, game history and cached answers in\n  %s\nand the daily run's log. Continue? [y/N] ", dir)
		in := bufio.NewScanner(os.Stdin)
		if !in.Scan() || !strings.EqualFold(strings.TrimSpace(in.Text()), "y") {
			fmt.Println("Nothing deleted.")
			return
		}
	}
	removed, err := player.Clean(dir)
	if err != nil {
		fail(err.Error())
	}
	if err := os.Remove(log); err == nil {
		removed = append(removed, log)
		// The log's folder is ours on Linux and Windows, not ~/Library/Logs.
		if filepath.Base(filepath.Dir(log)) == "wordle-solver" {
			os.Remove(filepath.Dir(log))
		}
	}
	if len(removed) == 0 {
		fmt.Println("No stored data to delete.")
	} else {
		fmt.Printf("Deleted %d files. The next game starts a new player.\n", len(removed))
	}
	if schedule.Installed() {
		fmt.Println("The daily run is still installed; remove it with: wordle-solver schedule -remove")
	}
}
