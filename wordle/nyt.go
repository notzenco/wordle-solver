package wordle

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const NYTBaseURL = "https://www.nytimes.com"

// DateForNumber returns the day of puzzle n, the number in the share text.
// Puzzle 0 ran on launch day, 19 June 2021, and one has run every day since.
func DateForNumber(n int) time.Time {
	return time.Date(2021, time.June, 19, 0, 0, 0, 0, time.Local).AddDate(0, 0, n)
}

type Puzzle struct {
	Number   int    `json:"days_since_launch"` // the number shown when sharing
	Solution string `json:"solution"`
	Date     string `json:"print_date"`
}

// FetchPuzzle gets the published puzzle for day from baseURL (NYTBaseURL in use).
func FetchPuzzle(ctx context.Context, baseURL string, day time.Time) (Puzzle, error) {
	var p Puzzle
	url := fmt.Sprintf("%s/svc/wordle/v2/%s.json", baseURL, day.Format(time.DateOnly))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return p, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return p, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return p, fmt.Errorf("no puzzle for %s (HTTP %d)", day.Format(time.DateOnly), resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return p, fmt.Errorf("reading puzzle for %s: %w", day.Format(time.DateOnly), err)
	}
	if len(p.Solution) != 5 {
		return p, fmt.Errorf("unexpected solution %q for %s", p.Solution, day.Format(time.DateOnly))
	}
	return p, nil
}
