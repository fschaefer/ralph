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

## Loop control

```bash
ralph 10 --timeout 180 --delay 3 -- claude -p "Continue until COMPLETE: true"
```

Important options:

- `--max-iterations <n>`: maximum loop count
- `--delay <s>`: pause between iterations
- `--timeout <s>`: limit one agent run
- `--stop-regex <expr>`: custom completion pattern
- `--resume`: continue from the last saved iteration
- `--dry-run`: print resolved config without running
- `--quiet`: reduce wrapper output

`RALPH_DELAY` and `STOP_REGEX` can provide defaults when the matching flags are not set.

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

These files allow resume support and make the last run inspectable.

## Examples

- [`examples/basic.sh`](examples/basic.sh): plain loop with an inline prompt
- [`examples/with-prompt-file.sh`](examples/with-prompt-file.sh): use an existing prompt file
- [`examples/with-prompt.sh`](examples/with-prompt.sh): generate a prompt from `--goal` and `--stack`
- [`examples/quiet.sh`](examples/quiet.sh): suppress wrapper noise
- [`examples/with-worktree.sh`](examples/with-worktree.sh): run in an isolated Git worktree
