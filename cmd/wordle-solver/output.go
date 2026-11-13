package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/notzenco/wordle-solver/internal/player"
	"github.com/notzenco/wordle-solver/internal/wordle"
)

// summary is one game on one line: number, score and coloured rows.
func summary(n int, turns []wordle.Turn) string {
	rows := make([]string, len(turns))
	for i, t := range turns {
		rows[i] = wordle.Emoji(t.Pattern)
	}
	return fmt.Sprintf("Wordle %-6s %s  %s", thousands(n), score(turns), strings.Join(rows, " "))
}

func score(turns []wordle.Turn) string {
	if len(turns) > 0 && turns[len(turns)-1].Pattern == wordle.Solved {
		return fmt.Sprintf("%d/6", len(turns))
	}
	return "X/6"
}

func printStats(store *player.Store) {
	st := store.Stats(wordle.NumberForDate(time.Now()))
	if st.Played == 0 {
		fmt.Fprintln(stdout, "No games played yet.")
		return
	}
	fmt.Fprintf(stdout, "Played %d   Win %% %d   Current streak %d   Max streak %d\n\n",
		st.Played, 100*st.Won/st.Played, st.CurrentStreak, st.MaxStreak)
	most := 1
	for _, c := range st.Guesses {
		most = max(most, c)
	}
	fmt.Fprintln(stdout, "Guess distribution")
	for turn := 1; turn <= 6; turn++ {
		c := st.Guesses[turn]
		fmt.Fprintf(stdout, "%d %s %d\n", turn, strings.Repeat("█", 1+30*c/most), c)
	}
}

// printGame shows the guesses with their colours, then the share text.
func printGame(number string, turns []wordle.Turn, toClipboard bool) {
	for _, t := range turns {
		fmt.Fprintf(stdout, "%s  %s\n", wordle.Emoji(t.Pattern), strings.ToUpper(t.Guess))
	}
	fmt.Fprintln(stdout)
	share := shareText(number, turns)
	fmt.Fprint(stdout, share)
	if toClipboard {
		copyText(share)
	}
}

func shareText(number string, turns []wordle.Turn) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Wordle %s%s\n\n", number, score(turns))
	for _, t := range turns {
		b.WriteString(wordle.Emoji(t.Pattern) + "\n")
	}
	return b.String()
}

func copyText(text string) {
	text = strings.TrimRight(text, "\n")
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		// clip reads UTF-16 with a byte-order mark correctly; plain UTF-8
		// would mangle the emoji.
		cmd = exec.Command("clip")
		text = utf16WithBOM(text)
	default:
		switch {
		case os.Getenv("WAYLAND_DISPLAY") != "":
			cmd = exec.Command("wl-copy")
		default:
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	}
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\ncould not copy to the clipboard: %v\n", err)
		return
	}
	fmt.Fprintln(stdout, "\n(copied to the clipboard)")
}

// thousands formats n with commas, as the share text does ("1,973").
func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func utf16WithBOM(s string) string {
	b := binary.LittleEndian.AppendUint16(nil, 0xFEFF)
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return string(b)
}
