package player

import (
	"fmt"
	"os"
	"path/filepath"
)

// mineFile holds the games a person plays themselves, apart from the
// player model's history so neither skews the other's statistics.
const mineFile = "my-games.json"

// LoadMine reads the games played by hand in dir, by puzzle number.
func LoadMine(dir string) (map[int]Game, error) {
	games := map[int]Game{}
	if err := readJSON(filepath.Join(dir, mineFile), &games); err != nil {
		return nil, err
	}
	if games == nil {
		games = map[int]Game{}
	}
	if err := checkGames(games); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, mineFile), err)
	}
	return games, nil
}

// SaveMine writes the games played by hand to dir.
func SaveMine(dir string, games map[int]Game) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, mineFile), games)
}

// StatsFor summarises games as Store.Stats does.
func StatsFor(games map[int]Game, today int) Stats {
	return (&Store{History: games}).Stats(today)
}
