// Command report reruns the strategy benchmark and regenerates every
// number and chart in docs/analysis.md from it:
//
//	go run ./tools/report
//
// It rewrites the sections of docs/analysis.md (and the headline table in
// README.md) between <!-- generated:NAME --> and <!-- /generated:NAME -->
// markers, and writes docs/results.json and the SVG charts in docs/img.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/notzenco/wordle-solver/internal/wordle"
)

type run struct {
	Strategy    string   `json:"strategy"`
	Hard        bool     `json:"hard"`
	Opener      string   `json:"opener"`
	Games       int      `json:"games"`
	Wins        [7]int   `json:"wins_by_guesses"` // index 1..6
	Failed      int      `json:"failed"`
	Mean        float64  `json:"mean_guesses_when_solved"`
	RuntimeSecs float64  `json:"runtime_seconds"`
	Positions   int      `json:"positions_decided"`
	FailedOn    []string `json:"failed_answers"`
}

func main() {
	docs := flag.String("docs", "docs", "folder holding analysis.md")
	readme := flag.String("readme", "README.md", "README whose headline table to rewrite")
	flag.Parse()

	var runs []run
	for _, hard := range []bool{false, true} {
		for _, st := range wordle.Strategies {
			r := wordle.Benchmark(st, hard, wordle.Answers)
			var failed []string
			for _, g := range r.Games {
				if g.Turns[len(g.Turns)-1].Pattern != wordle.Solved {
					failed = append(failed, strings.ToUpper(g.Answer))
				}
			}
			slices.Sort(failed)
			runs = append(runs, run{st.Name, hard, r.Opener, len(r.Games), r.Wins, r.Failed, r.Mean, r.Runtime.Seconds(), r.Positions, failed})
			fmt.Fprintf(os.Stderr, "%-9s hard=%-5v mean %.3f failed %d (%.2fs)\n", st.Name, hard, r.Mean, r.Failed, r.Runtime.Seconds())
		}
	}

	data, err := json.MarshalIndent(map[string]any{
		"command":     "go run ./tools/report",
		"go":          runtime.Version(),
		"platform":    runtime.GOOS + "/" + runtime.GOARCH,
		"cpus":        runtime.NumCPU(),
		"answers":     len(wordle.Answers),
		"max_guesses": 6,
		"runs":        runs,
	}, "", "  ")
	check(err)
	check(os.WriteFile(filepath.Join(*docs, "results.json"), append(data, '\n'), 0o644))

	img := filepath.Join(*docs, "img")
	check(os.MkdirAll(img, 0o755))
	check(os.WriteFile(filepath.Join(img, "distribution.svg"), []byte(distributionChart(runs)), 0o644))
	check(os.WriteFile(filepath.Join(img, "mean-guesses.svg"), []byte(meanChart(runs)), 0o644))
	check(os.WriteFile(filepath.Join(img, "runtime.svg"), []byte(runtimeChart(runs)), 0o644))

	path := filepath.Join(*docs, "analysis.md")
	doc, err := os.ReadFile(path)
	check(err)
	out := string(doc)
	for name, body := range map[string]string{
		"results":     resultsTable(runs),
		"headline":    headlineTable(runs),
		"openers":     openersTable(runs),
		"failures":    failuresList(runs),
		"environment": environment(),
	} {
		out = replaceSection(out, name, body)
	}
	check(os.WriteFile(path, []byte(out), 0o644))

	doc, err = os.ReadFile(*readme)
	check(err)
	check(os.WriteFile(*readme, []byte(replaceSection(string(doc), "headline", headlineTable(runs))), 0o644))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func replaceSection(doc, name, body string) string {
	re := regexp.MustCompile(`(?s)(<!-- generated:` + name + ` -->\n).*?(<!-- /generated:` + name + ` -->)`)
	if !re.MatchString(doc) {
		check(fmt.Errorf("no generated:%s section to rewrite", name))
	}
	return re.ReplaceAllLiteralString(doc, "<!-- generated:"+name+" -->\n"+body+"<!-- /generated:"+name+" -->")
}

func mode(hard bool) string {
	if hard {
		return "Hard"
	}
	return "Normal"
}

func failRate(r run) string {
	return fmt.Sprintf("%d (%.2f%%)", r.Failed, 100*float64(r.Failed)/float64(r.Games))
}

func resultsTable(runs []run) string {
	var b strings.Builder
	b.WriteString("| Strategy | Mode | Opener | Mean | 1 | 2 | 3 | 4 | 5 | 6 | Failed | Runtime | Positions |\n")
	b.WriteString("| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, r := range runs {
		fmt.Fprintf(&b, "| %s | %s | %s | %.3f |", r.Strategy, mode(r.Hard), strings.ToUpper(r.Opener), r.Mean)
		for turn := 1; turn <= 6; turn++ {
			fmt.Fprintf(&b, " %d |", r.Wins[turn])
		}
		fmt.Fprintf(&b, " %s | %.0f ms | %d |\n", failRate(r), 1000*r.RuntimeSecs, r.Positions)
	}
	return b.String()
}

func headlineTable(runs []run) string {
	byKey := map[string]run{}
	for _, r := range runs {
		byKey[r.Strategy+mode(r.Hard)] = r
	}
	var b strings.Builder
	b.WriteString("| Strategy | Mean guesses | Failed | Mean (Hard Mode) | Failed (Hard Mode) |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: |\n")
	for _, st := range wordle.Strategies {
		n, h := byKey[st.Name+"Normal"], byKey[st.Name+"Hard"]
		fmt.Fprintf(&b, "| %s | %.3f | %s | %.3f | %s |\n", st.Name, n.Mean, failRate(n), h.Mean, failRate(h))
	}
	return b.String()
}

func openersTable(runs []run) string {
	var b strings.Builder
	b.WriteString("| Strategy | Opener | Entropy (bits) | Expected answers left | Worst case left |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: |\n")
	for _, r := range runs {
		if r.Hard {
			continue // the first guess is the same in both modes
		}
		s := wordle.OpenerStats(r.Opener)
		fmt.Fprintf(&b, "| %s | %s | %.3f | %.1f | %d |\n", r.Strategy, strings.ToUpper(r.Opener), s.Bits, s.Expected, s.Worst)
	}
	return b.String()
}

func failuresList(runs []run) string {
	var b strings.Builder
	for _, r := range runs {
		if len(r.FailedOn) > 0 {
			fmt.Fprintf(&b, "- **%s, %s** (%d): %s\n", r.Strategy, strings.ToLower(mode(r.Hard)), len(r.FailedOn), strings.Join(r.FailedOn, ", "))
		}
	}
	return b.String()
}

func environment() string {
	return fmt.Sprintf("Generated with %s on %s/%s (%d CPUs), %d answers, at most 6 guesses.\n",
		runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), len(wordle.Answers))
}

// Charts. Colours follow a validated two-slot categorical palette (blue,
// orange) with matching dark-mode steps; text uses ink tokens, not series
// colours.

const style = `<style>
.surface{fill:#fcfcfb}.ink{fill:#0b0b0b}.ink2{fill:#52514e}.muted{fill:#898781}
.grid{stroke:#e1e0d9;stroke-width:1}.axis{stroke:#c3c2b7;stroke-width:1}
.s1{fill:#2a78d6}.s2{fill:#eb6834}.l1{stroke:#2a78d6}.l2{stroke:#eb6834}.ring{stroke:#fcfcfb}
text{font-family:system-ui,-apple-system,"Segoe UI",sans-serif;font-size:12px}
.title{font-size:15px;font-weight:600}.num{font-variant-numeric:tabular-nums}
@media (prefers-color-scheme:dark){
.surface{fill:#1a1a19}.ink{fill:#f0efec}.ink2{fill:#c3c2b7}.muted{fill:#898781}
.grid{stroke:#2c2c2a}.axis{stroke:#383835}
.s1{fill:#3987e5}.s2{fill:#d95926}.l1{stroke:#3987e5}.l2{stroke:#d95926}.ring{stroke:#1a1a19}}
</style>
`

func svgStart(b *strings.Builder, w, h int, title, desc string) {
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-labelledby="title desc">
<title id="title">%s</title><desc id="desc">%s</desc>
%s<rect class="surface" width="%d" height="%d" rx="8"/>
`, w, h, w, h, esc(title), esc(desc), style, w, h)
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// bar draws a vertical bar from baseline y0 up to y0-h with a rounded top.
func bar(b *strings.Builder, class string, x, y0, w, h float64, tip string) {
	if h <= 0 {
		return
	}
	r := math.Min(4, math.Min(w/2, h))
	fmt.Fprintf(b, `<path class="%s" d="M%.1f %.1fV%.1fQ%.1f %.1f %.1f %.1fH%.1fQ%.1f %.1f %.1f %.1fV%.1fZ"><title>%s</title></path>`+"\n",
		class, x, y0, y0-h+r, x, y0-h, x+r, y0-h, x+w-r, x+w, y0-h, x+w, y0-h+r, y0, esc(tip))
}

// hbar draws a horizontal bar from x0 rightwards with a rounded end.
func hbar(b *strings.Builder, class string, x0, y, w, h float64, tip string) {
	if w <= 0 {
		return
	}
	r := math.Min(4, math.Min(h/2, w))
	fmt.Fprintf(b, `<path class="%s" d="M%.1f %.1fH%.1fQ%.1f %.1f %.1f %.1fV%.1fQ%.1f %.1f %.1f %.1fH%.1fZ"><title>%s</title></path>`+"\n",
		class, x0, y, x0+w-r, x0+w, y, x0+w, y+r, y+h-r, x0+w, y+h, x0+w-r, y+h, x0, esc(tip))
}

func legend(b *strings.Builder, x, y float64, items ...[2]string) {
	for _, it := range items {
		fmt.Fprintf(b, `<rect class="%s" x="%.0f" y="%.0f" width="12" height="12" rx="3"/><text class="ink2" x="%.0f" y="%.0f">%s</text>`+"\n",
			it[0], x, y-10, x+18, y, esc(it[1]))
		x += 18 + 7*float64(len(it[1])) + 24
	}
}

// distributionChart is a grid of small histograms: one panel per strategy
// and mode, sharing a y scale.
func distributionChart(runs []run) string {
	const (
		panelW, panelH = 150.0, 130.0
		left, top      = 90.0, 92.0
		gapX, gapY     = 18.0, 54.0
		barW           = 14.0
	)
	cols := len(wordle.Strategies)
	w := int(left + float64(cols)*(panelW+gapX) + 6)
	h := int(top + 2*(panelH+gapY) + 4)
	most := 0
	for _, r := range runs {
		for _, c := range r.Wins {
			most = max(most, c)
		}
		most = max(most, r.Failed)
	}
	scale := (panelH - 24) / float64(most)

	var b strings.Builder
	svgStart(&b, w, h, "Guesses needed per strategy",
		"Histograms of games solved in 1 to 6 guesses, and games failed, for each strategy in normal and Hard Mode, over every answer.")
	b.WriteString(`<text class="ink title" x="20" y="30">Guesses needed per strategy, over every answer</text>` + "\n")
	legend(&b, 20, 58, [2]string{"s1", "solved in N guesses"}, [2]string{"s2", "failed (X)"})
	for i, r := range runs {
		row, col := 0, i%cols
		if r.Hard {
			row = 1
		}
		px := left + float64(col)*(panelW+gapX)
		py := top + float64(row)*(panelH+gapY)
		base := py + panelH
		if col == 0 {
			fmt.Fprintf(&b, `<text class="ink2" x="20" y="%.0f">%s</text>`+"\n", py+panelH/2, mode(r.Hard))
		}
		fmt.Fprintf(&b, `<text class="ink" x="%.0f" y="%.0f" font-weight="600">%s</text><text class="muted num" x="%.0f" y="%.0f" text-anchor="end">mean %.2f</text>`+"\n",
			px, py-6, r.Strategy, px+panelW, py-6, r.Mean)
		fmt.Fprintf(&b, `<line class="axis" x1="%.0f" x2="%.0f" y1="%.1f" y2="%.1f"/>`+"\n", px, px+panelW, base+0.5, base+0.5)
		slot := panelW / 7
		for k := 1; k <= 7; k++ {
			x := px + slot*float64(k-1) + (slot-barW)/2
			label := fmt.Sprint(k)
			count, class := 0, "s1"
			if k == 7 {
				label, count, class = "X", r.Failed, "s2"
			} else {
				count = r.Wins[k]
			}
			tip := fmt.Sprintf("%s, %s: %d games in %s", r.Strategy, strings.ToLower(mode(r.Hard)), count, label)
			if k == 7 {
				tip = fmt.Sprintf("%s, %s: %d games failed", r.Strategy, strings.ToLower(mode(r.Hard)), count)
			}
			bar(&b, class, x, base, barW, float64(count)*scale, tip)
			fmt.Fprintf(&b, `<text class="muted num" x="%.1f" y="%.0f" text-anchor="middle">%s</text>`+"\n", x+barW/2, base+16, label)
			if k == 7 && count > 0 {
				fmt.Fprintf(&b, `<text class="ink2 num" x="%.1f" y="%.1f" text-anchor="middle">%d</text>`+"\n", x+barW/2, base-float64(count)*scale-5, count)
			}
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}

// meanChart is a dot plot of mean guesses, normal against Hard Mode, with
// the failure counts beside each row.
func meanChart(runs []run) string {
	const (
		left, right, top, rowH = 110.0, 170.0, 96.0, 40.0
		width                  = 760.0
	)
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, r := range runs {
		lo, hi = math.Min(lo, r.Mean), math.Max(hi, r.Mean)
	}
	lo, hi = math.Floor(lo*10-0.5)/10, math.Ceil(hi*10+0.5)/10
	plotW := width - left - right
	x := func(v float64) float64 { return left + (v-lo)/(hi-lo)*plotW }
	n := len(wordle.Strategies)
	h := int(top + float64(n)*rowH + 40)

	var b strings.Builder
	svgStart(&b, int(width), h, "Mean guesses per strategy",
		"Dot plot of the mean number of guesses in solved games for each strategy, in normal and Hard Mode, with the number of failed games.")
	b.WriteString(`<text class="ink title" x="20" y="30">Mean guesses in solved games (lower is better)</text>` + "\n")
	legend(&b, 20, 58, [2]string{"s1", "Normal"}, [2]string{"s2", "Hard Mode"})
	axisY := top + float64(n)*rowH - rowH/2 + 14
	for v := lo; v <= hi+1e-9; v += 0.1 {
		fmt.Fprintf(&b, `<line class="grid" x1="%.1f" x2="%.1f" y1="%.0f" y2="%.0f"/><text class="muted num" x="%.1f" y="%.0f" text-anchor="middle">%.1f</text>`+"\n",
			x(v), x(v), top-rowH/2, axisY, x(v), axisY+16, v)
	}
	fmt.Fprintf(&b, `<text class="muted" x="%.0f" y="%.0f" text-anchor="end">failed: normal / hard</text>`+"\n", width-20, top-rowH/2-4)
	byKey := map[string]run{}
	for _, r := range runs {
		byKey[r.Strategy+mode(r.Hard)] = r
	}
	for i, st := range wordle.Strategies {
		y := top + float64(i)*rowH
		nr, hr := byKey[st.Name+"Normal"], byKey[st.Name+"Hard"]
		fmt.Fprintf(&b, `<text class="ink" x="20" y="%.0f">%s</text>`+"\n", y+4, st.Name)
		fmt.Fprintf(&b, `<line class="axis" x1="%.1f" x2="%.1f" y1="%.0f" y2="%.0f"/>`+"\n", x(nr.Mean), x(hr.Mean), y, y)
		// Identical means would hide one dot behind the other, so stack them.
		same := math.Abs(nr.Mean-hr.Mean) < 1e-9
		for j, p := range []struct {
			r     run
			class string
		}{{nr, "s1"}, {hr, "s2"}} {
			dy := 0.0
			if same {
				dy = float64(2*j-1) * 7
			}
			fmt.Fprintf(&b, `<circle class="%s ring" cx="%.1f" cy="%.0f" r="6" stroke-width="2"><title>%s, %s: mean %.3f guesses</title></circle>`+"\n",
				p.class, x(p.r.Mean), y+dy, st.Name, strings.ToLower(mode(p.r.Hard)), p.r.Mean)
		}
		if same {
			fmt.Fprintf(&b, `<text class="muted" x="%.1f" y="%.0f">same in both modes</text>`+"\n", x(nr.Mean)+14, y+4)
		}
		fmt.Fprintf(&b, `<text class="ink2 num" x="%.0f" y="%.0f" text-anchor="end">%d / %d</text>`+"\n", width-20, y+4, nr.Failed, hr.Failed)
	}
	b.WriteString("</svg>\n")
	return b.String()
}

// runtimeChart compares the time to play every answer, per strategy and mode.
func runtimeChart(runs []run) string {
	const (
		left, right, top, groupH, barH = 110.0, 90.0, 96.0, 52.0, 14.0
		width                          = 760.0
	)
	most := 0.0
	for _, r := range runs {
		most = math.Max(most, r.RuntimeSecs)
	}
	plotW := width - left - right
	n := len(wordle.Strategies)
	h := int(top + float64(n)*groupH + 10)
	byKey := map[string]run{}
	for _, r := range runs {
		byKey[r.Strategy+mode(r.Hard)] = r
	}

	var b strings.Builder
	svgStart(&b, int(width), h, "Runtime per strategy",
		"Bar chart of the time each strategy takes to play every answer, in normal and Hard Mode, with decisions cached by position.")
	b.WriteString(`<text class="ink title" x="20" y="30">Time to play every answer (decisions cached by position)</text>` + "\n")
	legend(&b, 20, 58, [2]string{"s1", "Normal"}, [2]string{"s2", "Hard Mode"})
	fmt.Fprintf(&b, `<line class="axis" x1="%.1f" x2="%.1f" y1="%.0f" y2="%.0f"/>`+"\n", left-0.5, left-0.5, top-8, top+float64(n)*groupH-12)
	for i, st := range wordle.Strategies {
		y := top + float64(i)*groupH
		fmt.Fprintf(&b, `<text class="ink" x="20" y="%.0f">%s</text>`+"\n", y+barH+2, st.Name)
		for j, hard := range []bool{false, true} {
			r := byKey[st.Name+mode(hard)]
			class := []string{"s1", "s2"}[j]
			by := y + float64(j)*(barH+2)
			bw := r.RuntimeSecs / most * plotW
			hbar(&b, class, left, by, bw, barH, fmt.Sprintf("%s, %s: %.0f ms, %d positions decided", st.Name, strings.ToLower(mode(hard)), 1000*r.RuntimeSecs, r.Positions))
			fmt.Fprintf(&b, `<text class="ink2 num" x="%.1f" y="%.1f">%.0f ms</text>`+"\n", left+math.Max(bw, 0)+6, by+barH-3, 1000*r.RuntimeSecs)
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}
