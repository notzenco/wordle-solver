// Package player keeps a persistent player profile and game history, so
// that games played on different days look like one consistent person's.
package player

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

const (
	profileFile = "profile.json"
	historyFile = "history.json"
	answersFile = "answers.json"
	// ConfigFile holds settings and saved providers, written by the CLI.
	ConfigFile = "config.json"
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
	// A hand-edited profile may hold words the solver can't score.
	s.Profile.Openers = slices.DeleteFunc(s.Profile.Openers, func(w string) bool { return !wordle.IsWord(w) })
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
	// A file holding null decodes to a nil map, which can't be added to.
	if s.History == nil {
		s.History = map[int]Game{}
	}
	if s.Answers == nil {
		s.Answers = map[int]string{}
	}
	if err := checkGames(s.History); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, historyFile), err)
	}
	for n, a := range s.Answers {
		if !wordle.IsWord(a) {
			return nil, fmt.Errorf("%s: answer %q for puzzle %d is not five letters a-z", filepath.Join(dir, answersFile), a, n)
		}
	}
	return s, nil
}

// checkGames rejects saved turns the solver can't score, such as hand edits.
func checkGames(games map[int]Game) error {
	for n, g := range games {
		for _, t := range g.Turns {
			if !wordle.IsWord(t.Guess) || len(t.Pattern) != 5 || strings.Trim(t.Pattern, "gyb") != "" {
				return fmt.Errorf("puzzle %d has a malformed turn %q %q", n, t.Guess, t.Pattern)
			}
		}
	}
	return nil
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

// Save writes the store. It re-reads the saved files under a lock and adds
// what this store holds, so two overlapping runs keep each other's games.
func (s *Store) Save() error {
	unlock, err := lock(s.dir)
	if err != nil {
		return err
	}
	defer unlock()
	var history map[int]Game
	var answers map[int]string
	if err := readJSON(filepath.Join(s.dir, historyFile), &history); err != nil {
		return err
	}
	if err := readJSON(filepath.Join(s.dir, answersFile), &answers); err != nil {
		return err
	}
	for n, g := range history {
		if _, ok := s.History[n]; !ok {
			s.History[n] = g
		}
	}
	for n, a := range answers {
		if _, ok := s.Answers[n]; !ok {
			s.Answers[n] = a
		}
	}
	if err := writeJSON(filepath.Join(s.dir, profileFile), s.Profile); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(s.dir, historyFile), s.History); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.dir, answersFile), s.Answers)
}

const lockFile = ".lock"

// lock takes dir's lock file, waiting up to ten seconds for another run to
// finish saving. A lock older than a minute is from a run that died, so it
// is taken over.
func lock(dir string) (func(), error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, lockFile)
	deadline := time.Now().Add(10 * time.Second)
	for {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return func() { os.Remove(p) }, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		if info, err := os.Stat(p); err == nil && time.Since(info.ModTime()) > time.Minute {
			os.Remove(p)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("another wordle-solver run is saving to %s; if none is, delete %s", dir, p)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Clean deletes the files a Store keeps in dir, then dir itself if nothing
// else is left in it. It removes only files it knows by name, since
// WORDLE_SOLVER_HOME may point at a folder that holds other things.
func Clean(dir string) (removed []string, err error) {
	for _, name := range []string{profileFile, historyFile, answersFile, mineFile, ConfigFile, lockFile} {
		temps, _ := filepath.Glob(filepath.Join(dir, name+"-*.tmp"))
		for _, p := range append([]string{filepath.Join(dir, name), filepath.Join(dir, name+".tmp")}, temps...) {
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

// writeJSON replaces path atomically so an interrupted save can't truncate
// it. The temporary file has a unique name, readable only by the owner.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
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
