// Package player keeps a persistent player profile and game history, so
// that games played on different days look like one consistent person's.
package player

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

const (
	profileFile = "profile.json"
	historyFile = "history.json"
	answersFile = "answers.json"
)

// Profile is the player's lasting habits.
type Profile struct {
	// Openers are favourite first words; the first is used about 80% of the time.
	Openers []string `json:"openers"`
	// SkipPastAnswers rules out words NYT has already used, as experienced
	// players do. New profiles get it half the time.
	SkipPastAnswers bool `json:"skip_past_answers"`
}

type Game struct {
	Date  string        `json:"date"`
	Turns []wordle.Turn `json:"turns"`
}

func (g Game) Solved() bool {
	return len(g.Turns) > 0 && g.Turns[len(g.Turns)-1].Pattern == wordle.Solved
}

type Store struct {
	Profile Profile
	History map[int]Game   // by puzzle number
	Answers map[int]string // NYT answers seen so far, by puzzle number
	dir     string
}

// Dir is $WORDLE_SOLVER_HOME, or wordle-solver in the user config directory.
func Dir() (string, error) {
	if d := os.Getenv("WORDLE_SOLVER_HOME"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "wordle-solver"), nil
}

// Load reads the store in dir, creating a new profile with rng on first use.
func Load(dir string, rng *rand.Rand) (*Store, error) {
	s := &Store{History: map[int]Game{}, Answers: map[int]string{}, dir: dir}
	if err := readJSON(filepath.Join(dir, profileFile), &s.Profile); err != nil {
		return nil, err
	}
	if len(s.Profile.Openers) == 0 {
		// Strong openers only, so careful play never has to swap one out.
		perm := rng.Perm(len(wordle.StrongOpeners))
		s.Profile.Openers = []string{wordle.StrongOpeners[perm[0]], wordle.StrongOpeners[perm[1]]}
		s.Profile.SkipPastAnswers = rng.Float64() < 0.5
	}
	if err := readJSON(filepath.Join(dir, historyFile), &s.History); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, answersFile), &s.Answers); err != nil {
		return nil, err
	}
	return s, nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (s *Store) Save() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(s.dir, profileFile), s.Profile); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(s.dir, historyFile), s.History); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.dir, answersFile), s.Answers)
}

// Clean deletes the files a Store keeps in dir, then dir itself if nothing
// else is left in it. It removes only files it knows by name, since
// WORDLE_SOLVER_HOME may point at a folder that holds other things.
func Clean(dir string) (removed []string, err error) {
	for _, name := range []string{profileFile, historyFile, answersFile} {
		for _, p := range []string{filepath.Join(dir, name), filepath.Join(dir, name+".tmp")} {
			err := os.Remove(p)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return removed, err
			}
			removed = append(removed, p)
		}
	}
	os.Remove(dir) // fails, as intended, while other files remain
	return removed, nil
}

// writeJSON replaces path atomically so an interrupted save can't truncate it.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) Game(number int) (Game, bool) {
	g, ok := s.History[number]
	return g, ok
}

func (s *Store) Record(number int, date string, turns []wordle.Turn) {
	s.History[number] = Game{Date: date, Turns: turns}
}

func (s *Store) AddAnswer(number int, answer string) { s.Answers[number] = answer }

// KnownAnswers lists the distinct NYT answers seen so far, sorted.
func (s *Store) KnownAnswers() []string {
	var words []string
	for _, w := range s.Answers {
		words = append(words, w)
	}
	slices.Sort(words)
	return slices.Compact(words)
}

// AnswersBefore lists the distinct cached answers of puzzles before number.
func (s *Store) AnswersBefore(number int) []string {
	var words []string
	for n, w := range s.Answers {
		if n < number {
			words = append(words, w)
		}
	}
	slices.Sort(words)
	return slices.Compact(words)
}

// Unplayed lists the puzzles to catch up on as of today: every day since the
// last game played, or just today for a new player.
func (s *Store) Unplayed(today int) []int {
	last := today - 1
	if len(s.History) > 0 {
		last = slices.Max(slices.Collect(maps.Keys(s.History)))
	}
	var out []int
	for n := last + 1; n <= today; n++ {
		out = append(out, n)
	}
	return out
}

// Missing lists the puzzle numbers from 0 to upto whose answers are unknown.
func (s *Store) Missing(upto int) []int {
	var out []int
	for n := 0; n <= upto; n++ {
		if _, ok := s.Answers[n]; !ok {
			out = append(out, n)
		}
	}
	return out
}

// Opener picks one of the favourite first words, mostly the first.
func (s *Store) Opener(rng *rand.Rand) string {
	if len(s.Profile.Openers) > 1 && rng.Float64() >= 0.8 {
		return s.Profile.Openers[1+rng.IntN(len(s.Profile.Openers)-1)]
	}
	return s.Profile.Openers[0]
}

// Target draws a finishing turn up to limit that pulls the player's record
// towards a typical spread: turns the record is short of are favoured, and
// turns it already has too many of are rarely drawn.
func (s *Store) Target(rng *rand.Rand, limit int) int {
	counts := map[int]int{}
	for _, g := range s.History {
		if g.Solved() {
			counts[len(g.Turns)]++
		}
	}
	games := float64(len(s.History) + 1)
	return wordle.DrawTurn(rng, limit, func(turn int) float64 {
		want := wordle.FinishWeights[turn]
		short := want*games - float64(counts[turn])
		// A small floor keeps every turn possible, as luck would.
		return max(short, 0) + 0.02*want
	})
}
