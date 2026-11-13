# A model of human play

Besides the strategies compared in [analysis.md](analysis.md), the solver
has a second mode: a simple model of how a person plays. `auto` and
`assist` (with the default `--by human`), `sim` and plain `bench` use it. It is a set of explicit, hand-set
assumptions about human habits, not something fitted to data, and this page
lists them, then compares the model's results with results people
reported.

The model lives in `internal/wordle/humanlike.go` (the `Solver` type), with
the player profile in `internal/player`.

## What the model assumes

### Vocabulary

People guess words they know. Every guess comes from the 2,315 common
answer words, never from the long tail of valid but obscure words, apart
from the popular opener ADIEU, which isn't an answer. When the
model chooses between similar options, it weights each word by how
familiar it is: (ln(1 + *f*) + 1)³, where *f* is the word's count in film
and TV subtitles (`internal/wordle/data/frequency.txt`). By that measure
HOUSE is about eight times as likely as NADIR.

### Favourite openers

People tend to open with the same word or two. The model has 24 popular
openers (ADIEU, AUDIO, CRANE, SLATE, STARE, RAISE, …). With a saved
profile, used by `auto`, it picks two favourites at random from
the 11 that split the answers well (STARE, SLATE, ROAST, HEART, …) and
opens with the first about 80% of the time and the second about 20%.
Without a profile (`sim`, `bench`), it picks any of the 24 at random.

Half of new profiles also rule out answers that were used on earlier days,
as some regular players do (`skip_past_answers` in `profile.json`).

### Mostly fitting guesses

Most guesses fit every clue so far, the way people play even outside Hard
Mode. The model plays a word that doesn't fit only for a reason a person
would recognise: to tell apart a group like FOUND/HOUND/MOUND/POUND, where
guessing one fitting word per turn could run out of guesses. Guesses are
ranked by how few answers they are expected to leave (the `expected`
strategy's score) and chosen among the better-ranked ones by familiarity.

### Slip-ups

People sometimes reuse a letter that is already ruled out, or put a yellow
letter back in the square where it was yellow. On each guess with at least
four guesses left, the model makes one such slip with probability 0.2
(`-sloppy`, from 0 to 1). A slip keeps every green letter and every letter
known to be in the answer, and makes exactly one mistake. The model picks
it by familiarity from the 8 slips that would still be most useful. Slips
never happen in Hard Mode or under a guess limit below six (`-max`), where
there is no room for them.

### The turn it finishes on

People don't all finish on the fastest possible turn. Before each game,
the model draws a target turn from these weights, cut off at the guess
limit, or uses `-target` if given:

| Target turn | 2 | 3 | 4 | 5 | 6 |
| --- | ---: | ---: | ---: | ---: | ---: |
| Weight | 4% | 27% | 40% | 22% | 7% |

These weights are a rough guess at a regular player's spread, set by hand.
They are not derived from the data below.

The target only slows the model down; it never speeds it up. While it is
ahead of its target, it plays the way a person who isn't sure yet might:

- with 12 or fewer candidates and three or more guesses left, a word that
  separates them instead of a likely answer;
- with one candidate left, one of the 4 closest near-misses instead of the
  answer;
- otherwise a reasonable fitting word rather than the best one.

Once it reaches the target, it plays its best-ranked fitting word (or a
splitting word in a trap like FOUND/HOUND/MOUND), apart from the occasional
slip. Near the end the look-ahead planner takes over: with two guesses left
it maximises the answers it is sure to solve (`bestForTwo`), and with three
left and more than three candidates it maximises those solved within three
(`bestForThree`). Over all 2,315 answers it lost no games in
`wordle-solver bench --seed 1`, `--seed 2` or `--seed 3` (means 4.29, 4.30 and
4.32 guesses).

## Compared with reported results

**Source.** COMAP, *2023 MCM Problem C: Predicting Wordle Results*, problem
statement and data file `2023_MCM_Problem_C_Data.xlsx`, from
<https://www.comap.org/membership/member-resources/item/predicting-wordle-results>,
accessed 13 November 2026. The statement describes the data as daily
results for 7 January to 31 December 2022, mined from scores players posted
on Twitter: for each day, the word, the number of reported results, and the
percentage solved in 1 to 6 tries or not solved (X). It notes that the
percentages "may not always sum to 100% due to rounding."

The data file is not included in this repository; COMAP's terms do not
allow redistributing modified copies. To reproduce the table, download the
problem data from the page above (`2023_MCM_Problem_C_Data.zip`, SHA-256
`4003df562d56030176cc2610e54a07c49c9fdfa9418b0abc0dfd6d14c01f21e1` on the
access date), unzip it and run:

```sh
go run ./tools/humanref -doc docs/human-model.md 2023_MCM_Problem_C_Data.xlsx
```

For every day in the file, the tool plays that day's word with the
human-like model (no profile, default settings, seeds 1 to 20) and with the
`entropy` strategy for reference, and tabulates both against the reported
percentages. It skips rows whose word is a typo not on the answer list and
rows whose percentages are far from 100, and lists them.

<!-- generated:human -->
| Source | Games | 1 | 2 | 3 | 4 | 5 | 6 | X | Mean when solved |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Reported, mean of daily percentages | 353 | 0.5% | 5.8% | 22.6% | 32.9% | 23.7% | 11.6% | 2.8% | 4.12 |
| Reported, weighted by results per day | 32,265,641 | 0.7% | 6.3% | 23.0% | 31.8% | 23.2% | 12.0% | 2.9% | 4.10 |
| Human-like solver (20 seeds per word) | 7,060 | 0.0% | 0.2% | 12.5% | 50.2% | 29.1% | 7.9% | 0.0% | 4.32 |
| `entropy` strategy | 353 | 0.0% | 2.8% | 48.4% | 43.1% | 5.7% | 0.0% | 0.0% | 3.52 |

353 days used from the data file; skipped 5 with not on the answer list: "rprobe", "naïve", "clen", "marxh", "tash", 1 with percentages sum outside 98-102: NYMPH (126%). The first row weights each day equally; the second counts every reported result.
<!-- /generated:human -->

### What the comparison shows

- **The model is slower on average than the people who posted results.**
  Its mean in solved games is 4.32 guesses, against 4.10–4.12 reported. The
  `entropy` strategy, at 3.52, is far faster than either.
- **Its spread is too narrow.** Half its games end on guess 4 (50.2%,
  against about 32–33% reported). It almost never finishes in two (0.2%
  against about 6%) and finishes in three half as often as people do
  (12.5% against about 23%).
- **It never loses, and people do.** About 3% of reported games were not
  solved, and 11.6–12.0% took six guesses, against 7.9% for the model. The
  model switches to its look-ahead planner near the end, which people
  can't do.
- **Why the 2s and 3s go missing.** The target weights put 31% of games on
  turns 2 and 3, but the model finishes only 12.7% there. The target can
  delay the model but never makes it faster, so when the clues after two
  guesses still leave several words, a game aimed at turn 3 ends on turn 4
  or later.

### Caveats

- The reported results are self-selected: they come from people who chose
  to post their score, and players who did well may be more likely to do
  so. They are a sample of people sharing results, not of everyone who
  played.
- The reported results mix normal and Hard Mode; the model played in
  normal mode.
- Each day counts once in the first reported row, while the model plays
  each day's word 20 times, so its row has less random variation.
- Matching the reported distribution more closely would mean fitting the
  target weights and slip rate to this data, which this project has not
  done.
