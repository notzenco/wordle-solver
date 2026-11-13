# wordle-solver

A Wordle solver whose games read like a decent human player's, not a bot's.

## Install

Download the archive for your system from the
[latest release](https://github.com/notzenco/wordle-solver/releases/latest)
(macOS, Linux and Windows, on Intel/AMD and ARM), unpack it and put
`wordle-solver` (or `wordle-solver.exe`) somewhere on your `PATH`.
`checksums.txt` lists each archive's SHA-256. The binaries are not signed:
on macOS, clear the download quarantine with
`xattr -d com.apple.quarantine wordle-solver`; on Windows, SmartScreen may
ask you to confirm the first run.

To build from source instead, with Go installed:

```sh
go install github.com/notzenco/wordle-solver/cmd/wordle-solver@latest
```

or, from a checkout, `go install ./cmd/wordle-solver`. Set `GOBIN` (for
example `GOBIN=~/.local/bin`) to choose where it goes.

`wordle-solver version` prints the version you have, and
`wordle-solver update` installs the latest release in place of the running
binary. It downloads the archive for your system from GitHub, checks it
against the release's `checksums.txt` and swaps the binary in with a rename,
so a failed update leaves the old one working. A build from a checkout has no
release version, so `update` leaves it alone unless you pass `-force`. If
the binary is in a folder you can't write to, rerun with permission to
change it (for example with `sudo`).

## Run

```sh

wordle-solver                     # solve today's NYT puzzle by itself
wordle-solver -date 2026-11-12    # solve another day's puzzle
wordle-solver --id 0              # solve by puzzle number (0 = CIGAR, 2021-06-19)
wordle-solver -from 1900          # solve every puzzle from #1900 to today (-to N to stop)
wordle-solver -catchup            # solve every day since your last game
wordle-solver schedule            # solve each day automatically (off until you run this)
wordle-solver clean               # delete your profile, history and cached answers
wordle-solver stats               # your NYT-style statistics
wordle-solver sync                # download every past answer (once; a few minutes)
wordle-solver -copy               # also copy the share grid to the clipboard
wordle-solver -hard               # only make guesses Hard Mode allows
wordle-solver -max 4              # always solve within 4 guesses
wordle-solver -target 4           # aim to finish on turn 4
wordle-solver -sloppy 0.4         # slip up more often (default 0.2, 0 = never)
wordle-solver play                # solve a puzzle with you
wordle-solver sim crazy           # watch it solve a known answer
wordle-solver bench               # result distribution over every answer
go test ./...                     # about 20 seconds: plays every answer
```

The default `auto` command looks up the day's answer from NYT's public
daily-puzzle endpoint (`/svc/wordle/v2/YYYY-MM-DD.json`). It then plays its
human-looking game against that answer and prints the guesses and the share
grid, under a header such as `Wordle #1,973 for 2026-11-13` whose number
works with `--id`. Dates follow your local time zone, as on the site. It
still plays a real game, using only the colours each guess earns; the answer
is used only to score those guesses.

NYT has added answers that aren't in the original list, such as MOXIE. Every
answer the solver fetches is cached, and `sync` downloads all past answers
at once. Cached answers join the solver's word list, so newer words are
ordinary candidates rather than surprises.

In `play`, type the suggested word into Wordle and enter the colours it
showed as `g`/`y`/`b` (or `2`/`1`/`0`). To play your own word instead,
enter `word pattern`, for example `crane bygbb`.

## One consistent player

`auto` and `play` play as the same person every day. On first use the
solver creates a profile with two favourite openers, used about 80/20. Each
`auto` game is saved, and each day's target turn is drawn to fill whatever
the running record is short of, so a month of games has a believable spread
rather than independent random results.

A real player only plays each puzzle once, so rerunning a day you have
already played shows the recorded game. `-fresh` plays it again and
replaces the record, and `-dry` neither reads nor saves anything.

`-from`/`-to` fill in a range of days in order, skipping days already
played. `stats` shows games played, win %, current and best streak and the
guess spread, as the site does.

Some players know NYT's past answers and skip them. New profiles get this
habit half the time (`skip_past_answers` in `profile.json`, which you can
change). Such a player rules out answers from before the puzzle being solved,
using whatever `sync` and earlier games have cached. If the clues leave
nothing else, the old answers come back, so a repeated answer is still found.

The profile, history and answer cache live in a `wordle-solver` folder in
the user config directory:

| System | Folder |
| --- | --- |
| macOS | `~/Library/Application Support/wordle-solver` |
| Linux | `~/.config/wordle-solver` (or under `$XDG_CONFIG_HOME`) |
| Windows | `%AppData%\wordle-solver` |

Set `WORDLE_SOLVER_HOME`, or pass `-home DIR`, to use another folder.

`wordle-solver clean` deletes that data and the daily run's log, after
asking (`-y` skips the question), so the next game starts a new player. It
removes only the files the solver writes, so other files in a custom
`WORDLE_SOLVER_HOME` folder are kept. It leaves a daily run in place; remove
that with `schedule -remove`.

## Daily automatic play

Nothing runs on its own until you ask: `wordle-solver schedule` sets up a
daily run with your system's own scheduler. Each day at 00:05 local time
(`-at HH:MM` to change it) it runs `wordle-solver auto -catchup`, which plays
every day since your last game, so the streak survives a machine that was
off or asleep. Run `wordle-solver schedule -remove` to stop it. Rerun
`schedule` after moving the binary, as the job records its path.

| System | Scheduler | Log |
| --- | --- | --- |
| macOS | launchd job `~/Library/LaunchAgents/wordle-solver.daily.plist`; runs a missed job on wake | `~/Library/Logs/wordle-solver.log` |
| Linux | systemd user timer `wordle-solver.timer` with `Persistent=true` | `~/.local/state/wordle-solver/wordle-solver.log` |
| Windows | Task Scheduler task `wordle-solver daily`, set to run as soon as possible after a missed start | `%LocalAppData%\wordle-solver\wordle-solver.log` |

On Linux the timer runs while you are logged in; `loginctl enable-linger`
lets it run when you are not. Without systemd, run
`wordle-solver auto -catchup -log FILE` daily from cron instead. `-copy`
uses `pbcopy` on macOS, `clip` on Windows, and `wl-copy` or `xclip` on Linux.

## Releases

Versions follow [semantic versioning](https://semver.org) and come from the
[Conventional Commit](https://www.conventionalcommits.org) messages on
`main`. [release-please](https://github.com/googleapis/release-please) keeps
a release PR open with the next version and changelog; merging it tags the
release, and the release workflow attaches binaries for every platform.
Before 1.0, `feat` commits raise the minor version and `fix` commits the
patch version.

## What makes it look human

- Guesses come only from the ~2,300 common answer words, plus NYT's newer
  answers once cached. When choosing
  between similar options it favours everyday words, the way a person thinks
  of HOUSE long before NADIR.
- Most guesses fit every clue so far (Hard Mode style). It also plays
  elimination words the way people do, such as one word to tell
  FOUND/HOUND/MOUND/POUND apart.
- Now and then it slips: it reuses a letter already ruled out, or puts a
  yellow back where it was already yellow, while keeping every green. Slips
  only happen with four or more guesses left, at the default limit of 6.
- It paces itself towards each game's target turn. When ahead, it spends
  turns on elimination words or near-misses instead of posting a run of 2s
  and 3s.

## Limits and Hard Mode

`-max N` (3–6, default 6) is a hard limit; `-target` is only an aim. Under a
limit below 6, or with `-hard`, the solver plays carefully: strong openers,
no slips or pacing detours, and its sharpest guess every turn. With three
guesses left it looks one guess ahead and picks whatever guarantees the most
answers. With two left it maximises the chance of finishing next turn.

Results over every answer:

| Mode | Mean | Over the limit or lost |
| --- | --- | --- |
| default | 4.3 | 0 |
| `-max 5` | — | about 1 in 20,000 (a guess-2 path into the _O_ER cluster) |
| `-max 4` | 3.7 | about 2.5% (even a perfect solver needs a fifth guess for about 1.6%) |
| `-hard` | 3.8 | about 0.3% lost, to traps like _ATCH and _O_ER |

## Project layout

| Path | What it holds |
| --- | --- |
| `cmd/wordle-solver` | The command: flags, subcommands and output |
| `internal/wordle` | Scoring, the human-looking solver and the NYT puzzle lookup |
| `internal/player` | The saved profile, game history, answer cache and statistics |
| `internal/schedule` | The daily run, via launchd, systemd or Task Scheduler |
| `internal/selfupdate` | The `update` command's download, checksum check and binary swap |
| `scripts/build-release.sh` | Builds the release archives; CI and the release workflow run it |

## Data and licence

The code is MIT licensed (see `LICENSE`).

- `internal/wordle/data/answers.txt` is the original public Wordle answer list.
- `internal/wordle/data/frequency.txt` holds counts for every valid five-letter
  Wordle word that appears in
  [FrequencyWords](https://github.com/hermitdave/FrequencyWords) (OpenSubtitles
  2018, by Hermit Dave), so later answers have counts too. It is licensed
  [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/), and so is
  this derived file.
