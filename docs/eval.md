# Evaluating models

The harness measures six-turn Wordle play from colour feedback. It records
wins, attempts, wasted turns, tokens reported by the server and elapsed time.
It makes one chat or decision request per turn, with no silent retries.

## Answer sets and contamination

The default `answers` set samples without replacement from answers.txt.
These public answers may have appeared in training data. The `heldout` set
samples frequency.txt words absent from answers.txt; these are held out from
the solver's original answer list, not guaranteed absent from model training.
Both sets accept every frequency word plus every original answer as guesses.
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

Replies accept a single five-letter word (optionally quoted) or a JSON
object with a guess field, optionally inside one Markdown code fence. Ambiguous prose is malformed. Malformed replies,
unknown words and Hard Mode violations waste a turn without adding feedback.
Request failures also waste a turn and are recorded; a recognised chat/decision
model mismatch stops the run instead. Hard Mode retains greens
and revealed letter counts, using the engine's HardValid rule.

Every response's reported model is retained. Any nonempty model differing
from the requested model marks the entire game as rerouted and excludes it
from eligible and solved totals, even if the provider uses an alias. Missing
model fields cannot establish whether routing occurred.

Without `--out`, each run writes a new file named after the provider, model
and start time in the `evals` folder beside the saved player data. The
output path is created exclusively so existing runs are preserved.
JSON lines begin with a `run` record (provider, credential-free base URL,
model, effective settings, seed, UTC date and tool version), followed by
`game` records and a final `summary` with the rerouted count. Each game has
the answer, success, attempts used, baseline count and success, difference,
error counts, token total, duration and individual attempts with raw replies,
reported models, patterns and problems. Durations are nanoseconds. Token
usage is provider-reported and zero when omitted. An interrupted file may
lack its summary. Stdout shows eligible games, wins and rerouting counts.

## Results

No model runs have been authorised for publication. Results are empty.

## Decision models and candidate options

Chat models generate a word; SystemOne decision models (Clef, Jev and Nimble)
choose from supplied options. Compare these kinds only with both using
`--options candidates`. Free-generation scores measure a different task:
filtering provides information and capping can omit the answer.

Built-in providers, and providers saved without `--kind`, detect the kind
per model. Requests start with the chat adapter. If the first chat request
of a run gets an HTTP 400 identifying a decision model, nothing has been
scored yet: the partial transcript is deleted, a notice goes to stderr, the
run restarts with `--kind systemone` and candidate options, and the run
record sets `kind_detected`. This happens at most once per run and never
after a game has been scored. An explicit kind, from `--kind`, the
`eval.kind` setting or a provider saved with `--kind`, is never changed: a
chat run against a decision model stops with a suggestion instead.
`--kind systemone` alone implies `--options candidates`.

```sh
wordle-solver eval --model omlx/clef-flash-4bit --games 3 --seed 42
wordle-solver eval --model omlx/clef-flash-4bit --max-options 64 --distractors 4 --games 3 --seed 42
```

SystemOne requires candidates mode. Each turn filters the full valid vocabulary
by the history, caps consistent words at `--max-options` (default 64), then
adds up to `--distractors N` inconsistent valid words and shuffles. No
inconsistent words exist before clues. Sampling uses the run seed plus turn
number and never consults the answer. Both adapters receive rules and plain-text
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
