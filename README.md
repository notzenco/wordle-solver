# wordle-solver

A Wordle solver whose games read like a decent human player's, not a bot's.

## Run

```sh
GOBIN=~/.local/bin go install .   # install as wordle-solver

wordle-solver                     # solve today's NYT puzzle by itself
wordle-solver -date 2026-11-12    # solve another day's puzzle
wordle-solver --id 0              # solve by puzzle number (0 = CIGAR, 2021-06-19)
wordle-solver -max 4              # always solve within 4 guesses
wordle-solver -target 4           # aim to finish on turn 4
wordle-solver -sloppy 0.4         # slip up more often (default 0.2, 0 = never)
wordle-solver play                # solve a puzzle with you
wordle-solver sim crazy           # watch it solve a known answer
wordle-solver bench               # result distribution over every answer
go test ./...                     # about 2 minutes: plays every answer
```

The default `auto` command looks up the day's answer from NYT's public
daily-puzzle endpoint (`/svc/wordle/v2/YYYY-MM-DD.json`). It then plays its
usual human-looking game against that answer and prints the guesses and the
share grid, under a header such as `Wordle #1,973 for 2026-11-13` whose
number works with `--id`. Dates follow your local time zone, as on the site. It still plays a real game, using only the colours each guess
gets; the answer is used only to score those guesses. Newer NYT answers that
are missing from the original list are added to the word list for that game.

In `play`, type the suggested word into Wordle and enter the colours it
showed as `g`/`y`/`b` (or `2`/`1`/`0`). To play your own word instead,
enter `word pattern`, for example `crane bygbb`.

## What makes it look human

- Guesses come only from the ~2,300 common answer words. Openers rotate
  through popular human picks such as CRANE, ADIEU and HOUSE.
- Most guesses fit every clue so far (hard-mode style). It also plays
  elimination words the way people do, such as one word to tell
  FOUND/HOUND/MOUND/POUND apart.
- Now and then it slips like a person: it reuses a letter already ruled
  out, or puts a yellow back where it was already yellow, while keeping every
  green. Slips only happen with four or more guesses left, and only at the
  default limit of 6. A tighter `-max` turns them off so the limit holds.
- Each game draws a target turn from a typical player's spread (mostly 3–5).
  When the solver gets ahead of that target, it spends turns on elimination
  words or near-misses instead of posting a run of 2s and 3s.

## Limits

`-max N` (3–6, default 6) is a hard limit; `-target` is only an aim. With
three guesses left the solver looks one guess ahead and picks whatever
guarantees the most answers. With two left it maximises the chance of
finishing next turn. On the last allowed guess it always plays a word that
fits. Over every answer:

| `-max` | over the limit |
| --- | --- |
| 6, 5 | 0 |
| 4 | about 2% (even a perfect solver needs a fifth guess for about 1.6%) |

Answer list: `wordle/data/answers.txt`, the original public Wordle answer list.
