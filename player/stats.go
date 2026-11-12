package player

import (
	"maps"
	"slices"
)

// Stats mirrors the statistics NYT shows a player.
type Stats struct {
	Played, Won   int
	CurrentStreak int
	MaxStreak     int
	Guesses       map[int]int // wins by number of guesses
}

// Stats summarises the history as of puzzle today. As on the site, the
// current streak still counts while today's puzzle is unplayed.
func (s *Store) Stats(today int) Stats {
	st := Stats{Guesses: map[int]int{}}
	run := 0
	prev := -2
	for _, n := range slices.Sorted(maps.Keys(s.History)) {
		g := s.History[n]
		st.Played++
		if n != prev+1 {
			run = 0
		}
		prev = n
		if g.Solved() {
			st.Won++
			st.Guesses[len(g.Turns)]++
			run++
			st.MaxStreak = max(st.MaxStreak, run)
		} else {
			run = 0
		}
	}

	n := today
	if _, played := s.History[today]; !played {
		n--
	}
	for ; ; n-- {
		g, ok := s.History[n]
		if !ok || !g.Solved() {
			break
		}
		st.CurrentStreak++
	}
	return st
}
