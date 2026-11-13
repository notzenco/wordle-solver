# Evaluating models

The harness measures six-turn Wordle play from colour feedback. It records
wins, attempts, wasted turns, tokens reported by the server and elapsed time.
It makes one chat completion request per turn, with no silent retries.

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

Settings live in config.json beside the saved player data. `-home` and
WORDLE_SOLVER_HOME select that directory. Precedence is flag, environment,
config, default. `settings` prints values and sources;
`settings set eval.games 20` saves a default and
`settings reset eval.games` removes it. Environment names start with
WORDLE_SOLVER_, uppercase the setting and replace dots and hyphens with
underscores (for example WORDLE_SOLVER_EVAL_MAX_TOKENS). Boolean flags use
`--hard=false` to override a true default.

```sh
wordle-solver providers
wordle-solver providers check ollama
wordle-solver models ollama
wordle-solver eval --model ollama/llama3.2 --games 10 --seed 42 --out ollama.jsonl
wordle-solver eval --model omlx/mlx-community/Llama-3.2-3B-Instruct-4bit --set heldout --seed 42 --out omlx.jsonl
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
object with a guess field. Ambiguous prose is malformed. Malformed replies,
unknown words and Hard Mode violations waste a turn without adding feedback.
Request failures also waste a turn and are recorded. Hard Mode retains greens
and revealed letter counts, using the engine's HardValid rule.

Every response's reported model is retained. Any nonempty model differing
from the requested model marks the entire game as rerouted and excludes it
from eligible and solved totals, even if the provider uses an alias. Missing
model fields cannot establish whether routing occurred.

The output path is created exclusively so existing runs are preserved.
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
