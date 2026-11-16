// Command evalreport rebuilds the results table in docs/eval.md from the
// committed eval transcripts:
//
//	go run ./tools/evalreport
//
// It reads every docs/eval-results/*.jsonl file and rewrites the section of
// docs/eval.md between <!-- generated:eval-results --> and
// <!-- /generated:eval-results -->.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	modeleval "github.com/notzenco/wordle-solver/internal/eval"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// labels names the published models and fixes their order in the table;
// other models follow, under their own ids.
var labels = []struct{ id, label string }{
	{"clef-flash-4bit", "Clef Flash, 4-bit"},
	{"clef-flash-8bit", "Clef Flash, 8-bit"},
	{"clef-8bit", "Clef, 8-bit"},
	{"Gemma 4 E2B · Heretic · 4-bit", "Gemma 4 E2B, 4-bit (chat)"},
}

// setOrder puts the official answers first, then the held-out sets.
var setOrder = map[string]int{"answers": 0, "heldout": 1, "matched": 2}

func setRank(set string) int {
	if r, ok := setOrder[set]; ok {
		return r
	}
	return len(setOrder)
}

type row struct {
	Model, Set                       string
	Eligible, Solved                 int
	Guesses, Baseline                int
	Duration                         time.Duration
	Probability, Loss, Uniform, Freq float64
	Scored                           int // turns whose decision scores exist
}

func readRun(path string) (row, error) {
	f, err := os.Open(path)
	if err != nil {
		return row{}, err
	}
	defer f.Close()
	var r row
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 16<<20)
	summary := false
	for sc.Scan() {
		var line struct {
			Type     string
			Model    string
			Settings map[string]any
			Game     struct {
				Guesses, Baseline int
				Duration          time.Duration
				Rerouted          bool
			}
			Eligible, Solved int
			Metrics          map[string]float64
		}
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			return r, fmt.Errorf("%s: %w", path, err)
		}
		switch line.Type {
		case "run":
			r.Model, _ = line.Settings["eval.set"].(string)
			r.Model, r.Set = line.Model, r.Model
		case "game":
			// Rerouted games are excluded from every eligible total.
			if !line.Game.Rerouted {
				r.Guesses += line.Game.Guesses
				r.Baseline += line.Game.Baseline
			}
			r.Duration += line.Game.Duration
		case "summary":
			summary = true
			r.Eligible, r.Solved = line.Eligible, line.Solved
			r.Probability, r.Loss = line.Metrics["true_probability"], line.Metrics["model_log_loss"]
			r.Uniform, r.Freq = line.Metrics["uniform_log_loss"], line.Metrics["frequency_log_loss"]
			r.Scored = int(line.Metrics["offered_answer_turns"])
		}
	}
	if err := sc.Err(); err != nil {
		return r, err
	}
	if !summary || r.Eligible == 0 {
		return r, fmt.Errorf("%s: no summary or no eligible games; an interrupted run can't be published", path)
	}
	return r, nil
}

func order(model string) int {
	for i, l := range labels {
		if l.id == model {
			return i
		}
	}
	return len(labels)
}

func label(model string) string {
	if i := order(model); i < len(labels) {
		return labels[i].label
	}
	return model
}

func table(rows []row) string {
	slices.SortStableFunc(rows, func(a, b row) int {
		if d := setRank(a.Set) - setRank(b.Set); d != 0 {
			return d
		}
		if a.Set != b.Set {
			return strings.Compare(a.Set, b.Set)
		}
		if d := order(a.Model) - order(b.Model); d != 0 {
			return d
		}
		return strings.Compare(a.Model, b.Model)
	})
	var b strings.Builder
	b.WriteString("| Model | Set | Solved | Guesses per game | Entropy strategy | Mean P(answer) | Log loss | Uniform | Frequency | Time |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, r := range rows {
		set := r.Set
		if set == "heldout" {
			set = "held-out"
		}
		n := float64(r.Eligible)
		lo, hi := modeleval.Wilson(r.Solved, r.Eligible)
		// Chat models give no probabilities, so they have no decision scores.
		scores := "– | – | – | –"
		if r.Scored > 0 {
			scores = fmt.Sprintf("%.1f%% | %.3f | %.3f | %.3f", 100*r.Probability, r.Loss, r.Uniform, r.Freq)
		}
		fmt.Fprintf(&b, "| %s | %s | %d/%d (%.0f–%.0f%%) | %.2f | %.2f | %s | %.0f s |\n",
			label(r.Model), set, r.Solved, r.Eligible, 100*lo, 100*hi, float64(r.Guesses)/n, float64(r.Baseline)/n,
			scores, r.Duration.Seconds())
	}
	return b.String()
}

func build(dir string) (string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return "", err
	}
	var rows []row
	for _, p := range paths {
		r, err := readRun(p)
		if err != nil {
			return "", err
		}
		rows = append(rows, r)
	}
	return table(rows), nil
}

var section = regexp.MustCompile(`(?s)(<!-- generated:eval-results -->\n).*?(<!-- /generated:eval-results -->)`)

func main() {
	results := flag.String("results", "docs/eval-results", "folder of eval transcripts")
	doc := flag.String("doc", "docs/eval.md", "document whose results table to rewrite")
	flag.Parse()
	body, err := build(*results)
	if err == nil {
		var b []byte
		if b, err = os.ReadFile(*doc); err == nil {
			s := strings.ReplaceAll(string(b), "\r\n", "\n")
			if !section.MatchString(s) {
				err = fmt.Errorf("%s has no generated:eval-results section", *doc)
			} else {
				s = section.ReplaceAllLiteralString(s, "<!-- generated:eval-results -->\n"+body+"<!-- /generated:eval-results -->")
				err = os.WriteFile(*doc, []byte(s), 0o644)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
