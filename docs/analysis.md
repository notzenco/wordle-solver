# Comparing guess strategies

This page compares five ways of choosing a guess. Each strategy plays every
one of the 2,315 answers, in normal mode and in Hard Mode, and the results
cover how many guesses it needs, how often it fails, and how long it takes.

Every number, table and chart below comes from one command, run from the
repository root:

```sh
go run ./tools/report
```

It replays every game, rewrites the generated sections of this file (between
`<!-- generated:… -->` markers), and writes [`results.json`](results.json)
and the charts in [`img/`](img). The strategies are deterministic, so
everything except the runtimes comes out the same on every machine. The
same comparison prints in the terminal with `wordle-solver bench --strategy all`.

<!-- generated:environment -->
Generated with go1.27.3 on darwin/arm64 (18 CPUs), 2315 answers, at most 6 guesses.
<!-- /generated:environment -->

## Headline

<!-- generated:headline -->
| Strategy | Mean guesses | Failed | Mean (Hard Mode) | Failed (Hard Mode) |
| --- | ---: | ---: | ---: | ---: |
| entropy | 3.495 | 0 (0.00%) | 3.582 | 12 (0.52%) |
| expected | 3.521 | 0 (0.00%) | 3.605 | 14 (0.60%) |
| minimax | 3.549 | 0 (0.00%) | 3.638 | 13 (0.56%) |
| freq | 3.650 | 22 (0.95%) | 3.650 | 22 (0.95%) |
| planner | 3.593 | 0 (0.00%) | 3.597 | 8 (0.35%) |
<!-- /generated:headline -->

"Mean guesses" averages the games that were solved; "Failed" counts games
not solved within six guesses.

## Method

**Game.** A guess earns one of 3⁵ = 243 colour patterns: green for the right
letter in the right place, yellow for a letter elsewhere in the answer, grey
otherwise, with repeated letters scored the way the game does. A game ends
when the guess is the answer or after six guesses.

**Words.** Both the possible answers and the allowed guesses are the 2,315
words of the original answer list (`internal/wordle/data/answers.txt`). The
game itself accepts about 13,000 guesses, many of them obscure. Restricting
guesses to answer words keeps every guess a real, common word, but it means
these results are not directly comparable with published optimal solvers
that use the full guess list. "Guessing from every valid word", under
Results, reruns the strategies with all 11,369 words this project knows
allowed as guesses; that is close to, but smaller than, the game's list.

**Hard Mode.** Every guess must keep each green letter in place and reuse
every revealed letter (`HardValid`). In normal mode any unplayed word may be
guessed.

**Implementation.** Before the timed runs, the benchmark computes the pattern
for every guess–answer pair once (a 2,315 × 2,315 table, about 5 MB) so that
splitting the remaining candidates is an array lookup. A strategy sees the
guesses so far and the candidates that still fit them. Strategies (a)–(d)
score every allowed guess and keep the best; ties go to a word that could
still be the answer (it might win now), then to the earlier word in the
list. Because every strategy is deterministic, the benchmark caches the
decision for each position (the sequence of guesses and patterns so far)
and reuses it in later games. The runtime is the time to play all 2,315
games with that cache, and "Positions" counts the decisions actually
computed. A test (`TestBenchmarkMatchesPlay`) checks that the cache never
changes a game.

### The strategies

With *n* candidates left, a guess splits them into groups of sizes
*n₁ … n₂₄₃*, one group per pattern.

| Name | Rule |
| --- | --- |
| (a) `entropy` | Maximise the information in the pattern, −Σ (*nᵢ*/*n*) log₂ (*nᵢ*/*n*) bits. |
| (b) `expected` | Minimise the candidates expected to remain, Σ *nᵢ*² / *n*, counting a win as none left. This is the score the human-like solver ranks guesses by. |
| (c) `minimax` | Minimise the largest group a pattern can leave; break ties by (b). |
| (d) `freq` | The classic hand heuristic: among the words that still fit, guess the one whose distinct letters appear in the most fitting words. It never looks at patterns. |
| (e) `planner` | The solver's look-ahead (`bestForThree`, `bestForTwo`). With three or more guesses left and more than three candidates, it picks, from the 8 best fitting words and the 8 best non-fitting words by (b), the guess that guarantees the most answers within three guesses. With two left, or three or fewer candidates, it maximises the answers it is sure to solve within two. Searching ahead from the first guess is too slow, so it opens with (b)'s choice. |

## Results

<!-- generated:results -->
| Strategy | Mode | Opener | Mean | 1 | 2 | 3 | 4 | 5 | 6 | Failed | Runtime | Positions |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| entropy | Normal | RAISE | 3.495 | 1 | 61 | 1139 | 1020 | 92 | 2 | 0 (0.00%) | 357 ms | 2479 |
| expected | Normal | RAISE | 3.521 | 1 | 61 | 1074 | 1089 | 90 | 0 | 0 (0.00%) | 387 ms | 2494 |
| minimax | Normal | RAISE | 3.549 | 1 | 65 | 1001 | 1159 | 89 | 0 | 0 (0.00%) | 615 ms | 2504 |
| freq | Normal | ALERT | 3.650 | 1 | 130 | 912 | 933 | 262 | 55 | 22 (0.95%) | 16 ms | 2293 |
| planner | Normal | RAISE | 3.593 | 1 | 120 | 918 | 1078 | 178 | 20 | 0 (0.00%) | 585 ms | 2392 |
| entropy | Hard | RAISE | 3.582 | 1 | 127 | 998 | 928 | 205 | 44 | 12 (0.52%) | 68 ms | 2313 |
| expected | Hard | RAISE | 3.605 | 1 | 126 | 953 | 961 | 221 | 39 | 14 (0.60%) | 68 ms | 2316 |
| minimax | Hard | RAISE | 3.638 | 1 | 124 | 900 | 1007 | 223 | 47 | 13 (0.56%) | 75 ms | 2322 |
| freq | Hard | ALERT | 3.650 | 1 | 130 | 912 | 933 | 262 | 55 | 22 (0.95%) | 16 ms | 2293 |
| planner | Hard | RAISE | 3.597 | 1 | 128 | 933 | 1016 | 195 | 34 | 8 (0.35%) | 238 ms | 2321 |
<!-- /generated:results -->

Columns 1–6 count the games won in that many guesses.

### Guessing from every valid word

Most published solvers may guess any of the valid words, not just the
answers. Allowed the same, the scoring strategies find better openers and
more escape words; the letter-frequency rule, which only plays fitting
answers, is unchanged. These runs use the same answers and are in
`results.json` as `runs_all_guesses`; `bench --strategy all --guesses all`
repeats them.

<!-- generated:all-guesses -->
| Strategy | Mode | Opener | Mean | 1 | 2 | 3 | 4 | 5 | 6 | Failed | Runtime | Positions |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| entropy | Normal | TARSE | 3.432 | 0 | 62 | 1247 | 952 | 52 | 2 | 0 (0.00%) | 1762 ms | 2478 |
| expected | Normal | TIARE | 3.477 | 0 | 66 | 1130 | 1068 | 51 | 0 | 0 (0.00%) | 1932 ms | 2503 |
| minimax | Normal | OLATE | 3.525 | 0 | 56 | 1041 | 1165 | 53 | 0 | 0 (0.00%) | 2991 ms | 2491 |
| freq | Normal | ALERT | 3.650 | 1 | 130 | 912 | 933 | 262 | 55 | 22 (0.95%) | 15 ms | 2293 |
| planner | Normal | TIARE | 3.555 | 0 | 129 | 958 | 1051 | 168 | 9 | 0 (0.00%) | 2178 ms | 2400 |
| entropy | Hard | TARSE | 3.528 | 0 | 116 | 1058 | 964 | 139 | 31 | 7 (0.30%) | 344 ms | 2377 |
| expected | Hard | TIARE | 3.574 | 0 | 104 | 1004 | 999 | 176 | 26 | 6 (0.26%) | 325 ms | 2403 |
| minimax | Hard | OLATE | 3.626 | 0 | 82 | 937 | 1079 | 186 | 26 | 5 (0.22%) | 367 ms | 2400 |
| freq | Hard | ALERT | 3.650 | 1 | 130 | 912 | 933 | 262 | 55 | 22 (0.95%) | 15 ms | 2293 |
| planner | Hard | TIARE | 3.568 | 0 | 124 | 954 | 1044 | 170 | 17 | 6 (0.26%) | 904 ms | 2362 |
<!-- /generated:all-guesses -->

![Histograms of guesses needed per strategy, normal and Hard Mode](img/distribution.svg)

![Mean guesses per strategy, normal and Hard Mode, with failure counts](img/mean-guesses.svg)

![Time to play every answer per strategy](img/runtime.svg)

### Best opener per strategy

The opener is the first guess a strategy makes from the empty position; it
is the same in both modes. The columns measure it against all 2,315 answers.

<!-- generated:openers -->
| Strategy | Opener | Entropy (bits) | Expected answers left | Worst case left |
| --- | --- | ---: | ---: | ---: |
| entropy | RAISE | 5.878 | 61.0 | 168 |
| expected | RAISE | 5.878 | 61.0 | 168 |
| minimax | RAISE | 5.878 | 61.0 | 168 |
| freq | ALERT | 5.746 | 71.6 | 196 |
| planner | RAISE | 5.878 | 61.0 | 168 |
<!-- /generated:openers -->

The three scoring rules agree on RAISE: among answer words it gives the most
information, leaves the fewest words on average and has the smallest worst
case. The letter-frequency heuristic picks ALERT, which covers common
letters but splits the answers less evenly.

### Games each strategy fails

<!-- generated:failures -->
- **freq, normal** (22): CATCH, CRACK, DADDY, FOYER, GONER, HATCH, HOLLY, JOLLY, POUND, PRIZE, RARER, SHAVE, STASH, TASTE, TATTY, TAUNT, TIGHT, VAUNT, WILLY, WITTY, WOOLY, WOUND
- **entropy, hard** (12): CRAZY, FOYER, GONER, GRAZE, MATCH, SWORE, TATTY, WASTE, WATCH, WATER, WIGHT, WILLY
- **expected, hard** (14): FOYER, GONER, GRAZE, MATCH, POUND, SHAVE, SWORE, TATTY, WASTE, WATCH, WATER, WIGHT, WILLY, WOUND
- **minimax, hard** (13): FOYER, GONER, GRAZE, MATCH, POUND, SHAPE, SHAVE, SWORE, TATTY, WASTE, WATCH, WATER, WOUND
- **freq, hard** (22): CATCH, CRACK, DADDY, FOYER, GONER, HATCH, HOLLY, JOLLY, POUND, PRIZE, RARER, SHAVE, STASH, TASTE, TATTY, TAUNT, TIGHT, VAUNT, WILLY, WITTY, WOOLY, WOUND
- **planner, hard** (8): FOYER, GONER, GRAZE, JAZZY, SWORE, WASTE, WIGHT, WOUND
<!-- /generated:failures -->

## Discussion

**Information is the best greedy rule here.** Maximising entropy gives the
lowest mean in both modes. Minimising the expected remaining words is close
behind, which is unsurprising: both reward an even split, and they differ
only in how heavily they weigh large groups. Minimax trades a little of the
mean for a guarantee about the worst case. None of the three fails in normal
mode, because each can spend a guess on a word that doesn't fit the clues
but separates the remaining candidates.

**Hard Mode removes that escape.** Every strategy that searches over guesses
fails 12–14 games in Hard Mode, and the failure lists show why: almost all
are families of words that differ in one or two letters, such as
_ATCH (MATCH, WATCH, PATCH), _OUND (POUND, WOUND), _O_ER (FOYER, GONER) and
_A_TE/_ATER (WASTE, WATER). Once the shared letters are revealed, every
legal guess must reuse them, so each guess tests only one or two new
letters. Avoiding these losses needs a different early guess, chosen with
the endgame in mind, which no one-step rule here does.

**A simple heuristic fails where the search strategies don't.** The
letter-frequency rule only ever guesses words that fit, so it plays the same
games in both modes and has no way out of a family like _ATCH; it fails 22
games (about 1%) even in normal mode. It is also by far the cheapest, since
it never computes a pattern.

**Looking ahead buys safety, not speed.** The planner maximises the answers
it is *sure* to solve within the next three guesses. That gives it the
fewest Hard Mode failures (8, against 12 for entropy), but in normal mode its mean is worse
than the other search strategies, with more games taking five or six
guesses. Its objective counts guaranteed wins and ignores how many guesses
a game takes, and it only considers 16 candidate guesses. That makes it a
good fit for the solver's own use, a tight guess limit (`--max`), where
finishing within the limit is all that matters.

**Cost.** With decisions cached by position, every strategy plays all 2,315
games in well under a second, computing a few thousand distinct positions.
In normal mode the three scoring rules and the planner take similar time.
In Hard Mode fewer guesses are legal after the first turn, so every search
strategy gets faster, the scoring rules much more than the planner.
Runtimes vary between machines and runs; the other numbers do not.

**Limits of this comparison.** All five strategies are greedy: they choose
one guess at a time and never search the whole game tree, so none of these
means is optimal. The main tables guess from the answer list; "Guessing
from every valid word" under Results allows every valid word. The benchmark measures each answer once, with equal weight, which is
how the answer list was designed to be used, not how often any word has
actually been chosen.
