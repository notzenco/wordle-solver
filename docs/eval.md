# Evaluating models

The harness measures six-turn Wordle play from colour feedback. It records
wins, attempts, wasted turns, tokens reported by the server and elapsed time.
It makes one chat or decision request per turn, with no silent retries.

## Answer sets and contamination

The default `answers` set samples without replacement from answers.txt.
These public answers may have appeared in training data. The `heldout` set
samples frequency.txt words absent from answers.txt; these are held out from
the solver's original answer list, not guaranteed absent from model training.
Held-out words are also much rarer than answers (median familiarity 71
against 620 on the solver's score), so a model doing worse on them may be
meeting obscure words rather than unseen ones. The `matched` set separates
the two: for each answer the same seed would draw from `answers`, it takes
the unused held-out word closest in familiarity. Up to a few hundred games
the medians match closely; over all 2,315 the familiar held-out words run
short and the median falls to 394. Many familiar held-out words are plurals
and verb forms (YEARS, TALKS), which the official answers avoid.
All sets accept every frequency word plus every original answer as guesses.
A fixed seed reproduces the sampled order; it does not guarantee deterministic
model output. The answer is never sent to the model.

The baseline is entropy, the lowest mean-guess strategy in
[the analysis](analysis.md) in both modes. Original answers use Strategy.Play
unchanged. Held-out answers use the same entropy scoring over the full
held-out candidate pool, with fitting words as guesses, without the original
answer-only pattern table. This baseline has different vocabulary constraints
from the model; compare the two sets separately. Beyond-baseline attempts
can be negative. A failed baseline is explicitly marked; its attempt count
is not a successful solution cost. Both players have six turns.

## Configuration and running

Settings live in config.json beside the saved player data. `--home` and
WORDLE_SOLVER_HOME select that directory. Precedence is flag, environment,
config, default. `settings` prints values and sources, `settings get KEY`
prints one value, `wordle-solver help settings` lists every key, and
`settings reset --all` clears them;
`settings set eval.games 20` saves a default and
`settings reset eval.games` removes it. Environment names start with
WORDLE_SOLVER_, uppercase the setting and replace dots and hyphens with
underscores (for example WORDLE_SOLVER_EVAL_MAX_TOKENS). Boolean flags use
`--hard=false` to override a true default.

```sh
wordle-solver providers
wordle-solver providers check ollama
wordle-solver models ollama
wordle-solver eval --model ollama/llama3.2 --games 10 --seed 42
wordle-solver eval --model omlx/mlx-community/Llama-3.2-3B-Instruct-4bit --set heldout --seed 42
wordle-solver eval --model omlx/mlx-community/Llama-3.2-3B-Instruct-4bit --set matched --seed 42
wordle-solver eval --model omniroute/your-model --games 10 --out router.jsonl
```

Set OMNIROUTE_API_KEY in your environment before using OmniRoute.
Providers store variable names only. Built-ins also cover LM Studio,
llama.cpp, OpenAI and OpenRouter. Listing and checking models send no prompts.
Eval sends prompts and may incur provider charges; run it deliberately.
The harness never starts or stops servers.

`providers add NAME --base-url http://localhost:9000/v1 --api-key-env MY_KEY`
saves a provider. Removing a saved built-in override restores the built-in.
For a one-off endpoint, use `eval --base-url URL --model bare-model`.
Model identifiers split at the first slash only.

## Turns, rerouting and transcripts

Eval scoring counts every reply: malformed replies, unknown words and Hard
Mode violations use a turn, so models are compared on the same terms.
`auto --by model` plays by the game's rules instead: such a word is
rejected without using a turn, up to three times per turn, and those
attempts are marked `Free` in the result.

Replies accept a single five-letter word (optionally quoted) or a JSON
object with a guess field, optionally inside one Markdown code fence. Ambiguous prose is malformed. Malformed replies,
unknown words and Hard Mode violations waste a turn without adding feedback.
Request failures also waste a turn; each attempt records the error, and the
summary counts them separately from malformed replies. A game in which every
request failed stops the run with a non-zero exit, so an outage is never
scored as model play. A recognised chat/decision model mismatch also stops the
run. Hard Mode retains greens
and revealed letter counts, using the engine's HardValid rule.

Every response's reported model is retained, including on malformed replies.
Any nonempty model differing from the requested model marks the entire game as
rerouted and excludes it from eligible and solved totals. Only two spellings
count as the same model: Ollama's default tag (`llama3.2:latest` for
`llama3.2`) and a dated snapshot suffix (`gpt-4o-2024-08-06` or
`name-20250101`); any other alias counts as rerouting. Missing model fields
cannot establish whether routing occurred.

Without `--out`, each run writes a new file named after the provider, model
and start time in the `evals` folder beside the saved player data. The
output path is created exclusively so existing runs are preserved.
JSON lines begin with a `run` record (provider, credential-free base URL,
model, effective settings, the fixed prompt text, seed, UTC date and tool
version), followed by `game` records and a final `summary` with the rerouted
and request-failure counts. Each game has
the answer, success, attempts used, baseline count and success, difference,
error counts, token total, duration and individual attempts with raw replies,
reported models, patterns, problems and error messages. Durations are nanoseconds. Token
usage is provider-reported and zero when omitted. Each game is written as
it ends, so an interrupted run keeps the games it finished but lacks the
summary. Stderr announces the run's request budget (six per game) and
reports each game as it ends; stdout shows the summary: eligible games, wins
and rerouting counts.

`--jobs N` plays up to N games at once (the first plays alone, in case it
reveals a decision model); the transcript keeps the sample order, so it is
the same as a run with one job. `--resume PATH` finishes an interrupted run:
it reads the settings from the run record, skips the games already written
and appends the rest and the summary. Only `--base-url`, `--api-key-env`
and `--jobs` may be given with it. A game lost to an outage is not written,
so resuming plays it again.

Game fields use the Go names shown above (`Answer`, `Solved`, `Guesses`,
`Attempts`, …). For example, the answers a run failed:

```sh
jq -r 'select(.type == "game" and (.game.Solved | not)) | .game.Answer' run.jsonl
```

## Results

Run on 2026-11-16 with v0.4.0: 200 games per row, seed 42, normal mode,
locally through oMLX on an Apple Silicon Mac. Clef, a decision model, chose
from up to 64 offered words that fit the clues (candidates mode); Gemma 4
E2B, a small chat model, replied with a word (free mode, temperature 0).
The same seed gives every model the same answers, so rows within a set
compare directly. Transcripts are in [eval-results](eval-results);
`go run ./tools/evalreport` rebuilds this table from them, and a test checks
the two still match.

<!-- generated:eval-results -->
| Model | Set | Solved | Guesses per game | Entropy strategy | Mean P(answer) | Log loss | Uniform | Frequency | Time |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Clef Flash, 4-bit | answers | 192/200 (92–98%) | 4.34 | 3.53 | 34.1% | 1.759 | 2.070 | 1.698 | 622 s |
| Clef Flash, 8-bit | answers | 190/200 (91–97%) | 4.21 | 3.53 | 33.7% | 1.793 | 2.080 | 1.682 | 645 s |
| Clef, 8-bit | answers | 192/200 (92–98%) | 4.28 | 3.53 | 36.7% | 1.854 | 2.005 | 1.591 | 2383 s |
| Gemma 4 E2B, 4-bit (chat) | answers | 1/200 (0–3%) | 5.97 | 3.53 | – | – | – | – | 306 s |
| Clef Flash, 4-bit | held-out | 173/200 (81–91%) | 4.67 | 4.25 | 30.9% | 2.327 | 1.961 | 2.800 | 711 s |
| Clef Flash, 8-bit | held-out | 175/200 (82–91%) | 4.69 | 4.25 | 31.8% | 2.207 | 1.906 | 2.769 | 788 s |
| Clef, 8-bit | held-out | 177/200 (83–92%) | 4.68 | 4.25 | 29.4% | 2.455 | 2.031 | 2.964 | 2694 s |
| Gemma 4 E2B, 4-bit (chat) | held-out | 0/200 (0–2%) | 6.00 | 4.25 | – | – | – | – | 290 s |
| Clef Flash, 4-bit | matched | 179/200 (84–93%) | 4.61 | 4.39 | 29.7% | 2.034 | 2.117 | 1.589 | 719 s |
| Clef Flash, 8-bit | matched | 175/200 (82–91%) | 4.67 | 4.39 | 29.1% | 2.037 | 2.072 | 1.561 | 792 s |
| Clef, 8-bit | matched | 174/200 (82–91%) | 4.67 | 4.39 | 28.6% | 2.162 | 2.175 | 1.641 | 2727 s |
| Gemma 4 E2B, 4-bit (chat) | matched | 0/200 (0–2%) | 6.00 | 4.39 | – | – | – | – | 285 s |
<!-- /generated:eval-results -->

The range after Solved is the 95% Wilson interval for the solve rate.
Guesses per game counts an unsolved game as 6, unlike the benchmark's means
in [analysis](analysis.md), which average solved games only, so the two
don't compare directly. Mean P(answer) and the log losses cover only the
turns where the answer was among the offered words (460–568 per Clef row);
lower log loss is better, and the uniform and frequency columns are the
baselines on those same turns. Chat models give no probabilities, so their
rows have none. Time is wall-clock time for the 200 games.

What the runs show:

- **On the official answers Clef is strong but not clever.** All three
  models solve 95–96% of games, 0.7–0.8 guesses per game behind the entropy
  strategy. Their answer probabilities beat a uniform guess but not a plain
  frequency prior: among the words that fit, picking the most familiar
  would have been better calibrated than Clef.
- **Words that were never answers cost about 8 points of solve rate.** On
  held-out words Clef solves 87–89%, and its log loss is worse than a
  uniform guess (the frequency prior is worse still, because these words
  are rare).
- **Familiarity doesn't explain the drop.** The matched set holds held-out
  words as familiar as the answers, and Clef does no better on it (87–90%)
  than on the plain held-out set; there the frequency prior beats Clef
  clearly. Clef seems to have learnt the character of the answer list, not
  just which words are common. One visible difference: 47% of matched words
  end in S, against 2.5% of sampled answers. Plurals are over-represented
  among the full Clef model's misses there (17 of 26) but not among Clef
  Flash's (8 of 21 and 8 of 25), so they explain only part of it.
- **Size and precision change nothing measurable.** The three Clef models'
  intervals overlap in every set, and the full model takes 3.4–3.8 times
  as long as Flash.
- **A small chat model can't play.** Gemma 4 E2B solved 1 game in 600. It
  mostly repeats AUDIO, TIGER or RIVER regardless of the colours, and
  68–73% of its guesses contradict clues it has already been given. Larger
  chat models that reason before answering need a much higher
  `--max-tokens` than the default 64 and were not run.

As expected with 64 of about 11,000 words offered, the answer was among the
first turn's options in 1 game per run.

To reproduce a row (oMLX serving the model locally):

```sh
wordle-solver eval --model omlx/clef-flash-4bit --set matched --games 200 --seed 42 --jobs 2
```

SystemOne requires candidates mode. Each turn filters the full valid vocabulary
by the history, caps consistent words at `--max-options` (default 64), then
adds up to `--distractors N` inconsistent valid words and shuffles. No
inconsistent words exist before clues. Sampling is seeded by the run seed,
the game's position in the run and the turn, so each game gets its own
options, and never consults the answer. Both adapters receive rules and plain-text
colour history without strategy recommendations. Each attempt records the exact
options; out-of-list choices are malformed. Consistent repeats remain allowed.

SystemOne replies retain confidence, probabilities and `usage.input_tokens`;
chat retains total tokens. Temperature and max-tokens affect chat only.
The request timeout applies to both; durations include HTTP overhead.

For eligible, non-rerouted games, decision metrics report mean true-answer
probability and mean natural-log loss only when the answer was offered.
Uniform loss is log(option count). Frequency-weighted loss weights each
offered word by the solver's familiarity score, (ln(1 + subtitle count) + 1)³,
normalised over that list. Raw subtitle counts were tried first and scored
worse than uniform on the answer list (4.67 against 4.16 nats with 64 options),
because a few very common words took nearly all the weight; familiarity scores
3.41. On the held-out set, whose words are rare by construction, every
frequency prior is worse than uniform, so compare it only within a set. All losses
use the same turns; lower is better. Zero probability is floored at 1e-15
for finite JSON. The qualifying turn count is reported; zero observations
means zero aggregates are unavailable, not perfect scores. Inconsistency rate
divides inconsistent valid choices by all valid offered choices, including
Hard Mode violations, excluding malformed replies and failed requests.
Probabilities must cover the options, lie in [0,1], and sum to one within
0.01 to allow rounded server values.

The [Clef MLX README](https://huggingface.co/mlx-community/clef-flash-4bit/blob/main/README.md)
documents a 16,384-token input budget, default state truncation and sequential
requests. The adapter sends `truncate: false`; servers must honour this extension
to guarantee history is preserved. Its reference latencies for 4-bit Flash on
an M5 Max are about 0.31 seconds at 1k tokens and 7 seconds at 16k. These are
measurements, not guarantees or time limits for oMLX or other hardware. Longer
states and option schemas increase tokens and latency; an option cap alone
does not guarantee a request fits.
