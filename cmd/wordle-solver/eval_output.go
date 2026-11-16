package main

import (
	"fmt"
	modeleval "github.com/notzenco/wordle-solver/internal/eval"
	"io"
	"text/tabwriter"
	"time"
)

type evalSummary struct {
	Model, Kind, Options, Out                               string
	Games, Eligible, Solved, Rerouted, RequestFailures      int
	Guesses, Beyond, Invalid, Malformed, Violations, Tokens int
	Duration                                                time.Duration
	MetricCount, Choices, Inconsistent                      int
	Probability, Loss, Uniform, Frequency                   float64
	Distractors                                             int
}

func writeEvalSummary(out io.Writer, s evalSummary) {
	fmt.Fprintf(out, "\n%s (%s, %s)\n\n", s.Model, s.Kind, s.Options)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	row := func(label string, value any) { fmt.Fprintf(w, "  %s\t%v\n", label, value) }
	if s.Eligible > 0 {
		lo, hi := modeleval.Wilson(s.Solved, s.Eligible)
		row("Solved", fmt.Sprintf("%d / %d eligible games (95%% CI %.0f–%.0f%%)", s.Solved, s.Eligible, 100*lo, 100*hi))
	} else {
		row("Solved", "0 / 0 eligible games")
	}
	row("Games", fmt.Sprintf("%d total, %d rerouted", s.Games, s.Rerouted))
	if s.Eligible > 0 {
		row("Guesses", fmt.Sprintf("%d total, %.2f per game", s.Guesses, float64(s.Guesses)/float64(s.Eligible)))
	} else {
		row("Guesses", "n/a (no eligible games)")
	}
	row("Beyond entropy", fmt.Sprintf("%+d total guesses", s.Beyond))
	tokenLabel := "Tokens"
	if s.Kind == "systemone" {
		tokenLabel = "Input tokens"
	}
	row(tokenLabel, thousands(s.Tokens))
	row("Time", s.Duration.Round(time.Millisecond))
	row("Invalid words", s.Invalid)
	row("Malformed replies", s.Malformed)
	row("Request failures", s.RequestFailures)
	row("Hard Mode violations", s.Violations)
	w.Flush()
	if s.Kind == "systemone" {
		fmt.Fprintln(out, "\nDecision scores (eligible games)")
		row("Answer offered", fmt.Sprintf("%d turns", s.MetricCount))
		if s.MetricCount > 0 {
			n := float64(s.MetricCount)
			row("Mean answer probability", fmt.Sprintf("%.1f%%", 100*s.Probability/n))
			row("Mean log loss", fmt.Sprintf("%.3f", s.Loss/n))
			row("Uniform baseline", fmt.Sprintf("%.3f", s.Uniform/n))
			row("Frequency baseline", fmt.Sprintf("%.3f", s.Frequency/n))
		} else {
			row("Probability / log loss", "n/a (answer never offered)")
		}
		if s.Distractors > 0 {
			if s.Choices > 0 {
				row("Inconsistent choices", fmt.Sprintf("%d / %d (%.1f%%)", s.Inconsistent, s.Choices, 100*float64(s.Inconsistent)/float64(s.Choices)))
			} else {
				row("Inconsistent choices", "n/a (no valid choices)")
			}
		}
		w.Flush()
		fmt.Fprintln(out, "  Log loss uses natural logs; lower is better.")
	}
	fmt.Fprintf(out, "\nTranscript: %s\n", s.Out)
}
