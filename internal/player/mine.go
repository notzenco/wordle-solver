package player

import (
	"fmt"
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

// SaveMine adds games to the hand-played games saved in dir, under the
// same lock as Store.Save.
func SaveMine(dir string, games map[int]Game) error {
	unlock, err := lock(dir)
	if err != nil {
		return err
	}
	defer unlock()
	saved := map[int]Game{}
	if err := readJSON(filepath.Join(dir, mineFile), &saved); err != nil {
		return err
	}
	for n, g := range saved {
		if _, ok := games[n]; !ok {
			games[n] = g
		}
	}
	return writeJSON(filepath.Join(dir, mineFile), games)
}

// StatsFor summarises games as Store.Stats does.
func StatsFor(games map[int]Game, today int) Stats {
	return (&Store{History: games}).Stats(today)
}
