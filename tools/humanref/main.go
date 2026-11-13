// Command humanref compares the human-like solver with results people
// reported in 2022, from the COMAP MCM 2023 Problem C data file:
//
//	go run ./tools/humanref -doc docs/human-model.md 2023_MCM_Problem_C_Data.xlsx
//
// The data file is not in this repository; docs/human-model.md says where
// to download it. For each day in the file, the solver plays that day's
// word with several fixed seeds, and the two distributions are printed side
// by side. With -doc, the table also replaces the
// <!-- generated:human --> section of that file.
package main

import (
	"archive/zip"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

// day is one row of the data file: the word and the percentage of reported
// results solved in 1..6 tries (pct[0..5]) or not solved (pct[6]).
type day struct {
	word     string
	reported int
	pct      [7]float64
}

func main() {
	doc := flag.String("doc", "", "markdown file whose generated:human section to rewrite")
	seeds := flag.Int("seeds", 20, "games per word for the human-like solver, one per seed")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: humanref [-doc FILE] [-seeds N] 2023_MCM_Problem_C_Data.xlsx")
		os.Exit(2)
	}
	days, skipped, err := readDays(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out := report(days, skipped, *seeds)
	fmt.Print(out)
	if *doc != "" {
		data, err := os.ReadFile(*doc)
		if err == nil {
			re := regexp.MustCompile(`(?s)(<!-- generated:human -->\n).*?(<!-- /generated:human -->)`)
			if !re.Match(data) {
				err = fmt.Errorf("%s has no generated:human section", *doc)
			} else {
				data = re.ReplaceAll(data, []byte("<!-- generated:human -->\n"+strings.ReplaceAll(out, "$", "$$")+"<!-- /generated:human -->"))
				err = os.WriteFile(*doc, data, 0o644)
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

// report plays the human-like solver and the entropy strategy on each day's
// word and returns a markdown table against the reported results.
func report(days []day, skipped map[string][]string, seeds int) string {
	var meanPct, weighted [7]float64
	total := 0
	for _, d := range days {
		for i, p := range d.pct {
			meanPct[i] += p / float64(len(days))
			weighted[i] += p * float64(d.reported)
		}
		total += d.reported
	}
	for i := range weighted {
		weighted[i] /= float64(total)
	}

	var model [7]float64
	games := 0
	for s := range seeds {
		rng := rand.New(rand.NewPCG(uint64(s+1), uint64(s+1)))
		for _, d := range days {
			model[bucket(wordle.NewSolver(rng, 0, 6).Solve(d.word))]++
			games++
		}
	}
	for i := range model {
		model[i] *= 100 / float64(games)
	}

	var best [7]float64
	entropy, _ := wordle.StrategyByName("entropy")
	for _, d := range days {
		best[bucket(entropy.Play(d.word, false))]++
	}
	for i := range best {
		best[i] *= 100 / float64(len(days))
	}

	var b strings.Builder
	b.WriteString("| Source | Games | 1 | 2 | 3 | 4 | 5 | 6 | X | Mean when solved |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	line := func(name string, games int, p [7]float64) {
		fmt.Fprintf(&b, "| %s | %s |", name, thousands(games))
		for _, v := range p {
			fmt.Fprintf(&b, " %.1f%% |", v)
		}
		fmt.Fprintf(&b, " %.2f |\n", meanSolved(p))
	}
	line("Reported, mean of daily percentages", len(days), meanPct)
	line("Reported, weighted by results per day", total, weighted)
	line(fmt.Sprintf("Human-like solver (%d seeds per word)", seeds), games, model)
	line("`entropy` strategy", len(days), best)
	fmt.Fprintf(&b, "\n%d days used from the data file", len(days))
	if len(skipped) > 0 {
		var parts []string
		for _, reason := range []string{"not on the answer list", "percentages sum outside 98-102"} {
			if words := skipped[reason]; len(words) > 0 {
				parts = append(parts, fmt.Sprintf("%d with %s: %s", len(words), reason, strings.Join(words, ", ")))
			}
		}
		fmt.Fprintf(&b, "; skipped %s", strings.Join(parts, ", "))
	}
	b.WriteString(". The first row weights each day equally; the second counts every reported result.\n")
	return b.String()
}

func bucket(turns []wordle.Turn) int {
	if turns[len(turns)-1].Pattern == wordle.Solved {
		return len(turns) - 1
	}
	return 6
}

func meanSolved(p [7]float64) float64 {
	sum, won := 0.0, 0.0
	for i, v := range p[:6] {
		sum += float64(i+1) * v
		won += v
	}
	return sum / won
}

func thousands(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// readDays reads the data file's first sheet. Row 2 holds the headers;
// column D is the word, E the number of reported results and G..M the
// percentages for 1..6 tries and X. Rows whose word is not on the answer
// list (the file has typos such as CLEN), or whose percentages are far
// from 100, are skipped and listed.
func readDays(path string) ([]day, map[string][]string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, err
	}
	defer zr.Close()
	var strs struct {
		SI []struct {
			T string `xml:"t"`
		} `xml:"si"`
	}
	var sheet struct {
		Rows []struct {
			R     int `xml:"r,attr"`
			Cells []struct {
				Ref  string `xml:"r,attr"`
				Type string `xml:"t,attr"`
				V    string `xml:"v"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	for name, v := range map[string]any{"xl/sharedStrings.xml": &strs, "xl/worksheets/sheet1.xml": &sheet} {
		f, err := zr.Open(name)
		if err != nil {
			return nil, nil, err
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, nil, err
		}
		if err := xml.Unmarshal(data, v); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
	}

	var days []day
	skipped := map[string][]string{}
	five := regexp.MustCompile(`^[a-z]{5}$`)
	for _, row := range sheet.Rows {
		if row.R < 3 {
			continue
		}
		var d day
		ok := true
		for _, c := range row.Cells {
			col := strings.TrimRight(c.Ref, "0123456789")
			if col == "D" {
				if i, err := strconv.Atoi(c.V); err == nil && c.Type == "s" && i < len(strs.SI) {
					d.word = strings.ToLower(strings.TrimSpace(strs.SI[i].T))
				}
				continue
			}
			n, err := strconv.ParseFloat(c.V, 64)
			switch {
			case col == "E":
				d.reported = int(n)
				ok = ok && err == nil
			case len(col) == 1 && col[0] >= 'G' && col[0] <= 'M':
				d.pct[col[0]-'G'] = n
				ok = ok && err == nil
			}
		}
		if d.word == "" && d.reported == 0 {
			continue // a blank row
		}
		sum := 0.0
		for _, p := range d.pct {
			sum += p
		}
		switch {
		case !five.MatchString(d.word) || !wordle.InAnswers(d.word):
			skipped["not on the answer list"] = append(skipped["not on the answer list"], strconv.Quote(d.word))
		case !ok || math.Abs(sum-100) > 2:
			skipped["percentages sum outside 98-102"] = append(skipped["percentages sum outside 98-102"], fmt.Sprintf("%s (%.0f%%)", strings.ToUpper(d.word), sum))
		default:
			days = append(days, d)
		}
	}
	return days, skipped, nil
}
