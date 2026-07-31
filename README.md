# ralph

`ralph` is a small loop runner for AI coding agents.

It repeatedly executes an agent command until either:

- the agent outputs the configured completion signal, or
- the maximum number of iterations is reached.

## Quickstart

Run a plain loop with an explicit prompt:

```bash
ralph 5 -- claude -p "Fix the failing tests and print COMPLETE: true when done"
```

By default, `ralph` stops when the agent prints:

```text
COMPLETE: true
```

## Prompt modes

`ralph` supports two prompt modes.

### 1. Use an existing prompt file

```bash
ralph 5 --prompt-file prompts/task.md -- claude -p @{PROMPT_FILE}
```

Use this when you already have a hand-written prompt and want full control.

### 2. Generate a prompt from goal and stack

```bash
ralph \
  8 \
  --goal "Build a REST API for managing users" \
  --stack "Go, chi, SQLite" \
  -- claude -p @{PROMPT_FILE}
```

This generates `.ralph/PROMPT.md` from the built-in template. If `PROMPT_TEMPLATE.md`
exists in the working directory, it overrides the built-in template.
Use `@{PROMPT_FILE}` in the agent command to pass the generated prompt to the agent.

Rules:

- `--prompt-file` cannot be combined with `--goal` or `--stack`
- `--stack` requires `--goal`
- `{PROMPT_FILE}` requires one of the prompt modes above

## OpenAI-compatible providers with the Copilot SDK

`--copilot-sdk` supports GitHub Copilot by default. Set
`COPILOT_PROVIDER_BASE_URL` to use an OpenAI-compatible provider instead;
GitHub authentication is then not required. Provider credentials are read only
from the process environment and are never written to `.ralph/`.

```bash
export COPILOT_PROVIDER_TYPE=openai
export COPILOT_PROVIDER_BASE_URL=https://provider.example/v1
export COPILOT_PROVIDER_API_KEY=your-api-key
export COPILOT_PROVIDER_WIRE_API=completions
export COPILOT_MODEL=your-model

ralph 8 --timeout 1800 --copilot-sdk
```

Optional variables are `COPILOT_PROVIDER_MODEL_ID`,
`COPILOT_PROVIDER_WIRE_MODEL`, `COPILOT_PROVIDER_MAX_PROMPT_TOKENS`,
`COPILOT_PROVIDER_MAX_OUTPUT_TOKENS`, `COPILOT_PROVIDER_BEARER_TOKEN`, and
`COPILOT_PROVIDER_TRANSPORT`. With a custom provider, set either
`COPILOT_MODEL` or `--model`; the explicit CLI flag takes precedence.

## Loop control

```bash
ralph 10 --timeout 180 --delay 3 -- claude -p "Continue until COMPLETE: true"
```

Important options:

- `--max-iterations <n>`: maximum loop count
- `--delay <s>`: pause between iterations
- `--timeout <s>`: limit one agent run
- `--stop-regex <expr>`: custom completion pattern
- `--dry-run`: print resolved config without running
- `--quiet`: reduce wrapper output
- `--clean-all`: remove the entire `.ralph/` directory
- `--clean`: remove worktrees from previous runs in `.ralph/worktrees/`


## Worktree isolation

```bash
ralph 10 --worktree --goal "Refactor auth module" --stack "Python, FastAPI" \
  -- claude -p @{PROMPT_FILE}
```

With `--worktree`, `ralph` creates a dedicated Git worktree so the agent does not
modify your current checkout directly.

- Worktree path: `.ralph/worktrees/<timestamp>/`
- Branch: `ralph/run-<timestamp>`

## Runtime files

`ralph` stores run state in `.ralph/`:

- `.ralph/PROMPT.md`
- `.ralph/iteration.txt`
- `.ralph/last-output.txt`
- `.ralph/ralph.log`

These files make the last run inspectable.

## Cleanup

```bash
ralph --clean-all            # Remove entire .ralph/ directory
ralph --clean               # Remove worktrees from previous runs only
```

`--clean-all` removes the whole `.ralph/` runtime directory (logs, prompts,
worktrees, iteration state). This is useful to fully reset before a new run.
`--clean` only removes stale git worktrees from previous `--worktree` runs.
Both flags work without an agent command.

## Examples

- [`examples/basic.sh`](examples/basic.sh): plain loop with an inline prompt
- [`examples/with-prompt-file.sh`](examples/with-prompt-file.sh): use an existing prompt file
- [`examples/with-prompt.sh`](examples/with-prompt.sh): generate a prompt from `--goal` and `--stack`
- [`examples/quiet.sh`](examples/quiet.sh): suppress wrapper noise
- [`examples/with-worktree.sh`](examples/with-worktree.sh): run in an isolated Git worktree
