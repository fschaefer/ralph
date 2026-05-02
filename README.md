# ralph 🤖

A minimal Bash loop runner for autonomous AI coding agents. Give it an agent command and it will keep calling it until the agent signals completion or the iteration limit is reached.

Inspired by [wiggum-cli](https://github.com/federiconeri/wiggum-cli).

---

## Installation

```bash
git clone https://github.com/youruser/ralph
cd ralph
chmod +x ralph.sh
```

No dependencies beyond `bash` (≥ 4) and standard POSIX tools. Optionally requires `git` for `--resume`, `--worktree`, and run summaries.

---

## Usage

```
./ralph.sh [options] [iterations] -- <agent-command...>
```

The `--` separator is **required** to distinguish ralph flags from the agent command.

### Quickstart

```bash
# Run an agent up to 5 times, stop when it prints "COMPLETE: true"
./ralph.sh 5 -- claude -p @.ralph/PROMPT.md

# Same, using --max-iterations
./ralph.sh --max-iterations 5 -- claude -p @.ralph/PROMPT.md
```

---

## Options

| Flag | Default | Description |
|---|---|---|
| `--max-iterations <n>` | `5` | Maximum number of loop iterations |
| `--delay <s>` | `2` (or `$RALPH_DELAY`) | Pause between iterations in seconds |
| `--stop-regex <pattern>` | `^COMPLETE:\s*true$` (or `$STOP_REGEX`) | Regex that triggers a successful stop |
| `--dry-run` | off | Print configuration and exit without running |
| `--resume` | off | Resume from last saved iteration (`.ralph/iteration.txt`) |
| `--worktree` | off | Run the agent inside an isolated Git worktree |
| `--goal <text>` | – | Project goal – fills `{{GOAL}}` in `PROMPT_TEMPLATE.md` |
| `--stack <text>` | – | Tech stack – fills `{{STACK}}` in `PROMPT_TEMPLATE.md` |
| `--prompt-file <path>` | – | Use a ready-made prompt file (overrides `--goal`/`--stack`) |
| `-h`, `--help` | – | Show help and exit |

---

## PROMPT Integration

ralph can auto-generate a structured agent prompt from `PROMPT_TEMPLATE.md` by substituting two placeholders:

| Placeholder | Flag |
|---|---|
| `{{GOAL}}` | `--goal "..."` |
| `{{STACK}}` | `--stack "..."` |

The filled template is saved to `.ralph/PROMPT.md`. Use `{PROMPT_FILE}` anywhere in your agent command to refer to it:

```bash
./ralph.sh \
  --goal "Build a REST API with CRUD endpoints for users" \
  --stack "Node.js, Express, SQLite" \
  10 -- claude -p @{PROMPT_FILE}
```

You can also provide a hand-crafted prompt file directly:

```bash
./ralph.sh --prompt-file my-prompt.md 10 -- claude -p @{PROMPT_FILE}
```

### Minimal prompt for the agent

The agent only needs to output `COMPLETE: true` on its own line when all tasks are done. Everything else – task tracking, progress log, git commits – is managed by the agent itself via `tasks.md` and `progress.txt` (as defined in `PROMPT_TEMPLATE.md`).

---

## Worktree Isolation

`--worktree` creates a dedicated Git worktree for each run, so the agent never touches your working tree:

```bash
./ralph.sh --worktree --goal "Refactor auth module" --stack "Python, FastAPI" \
  10 -- claude -p @{PROMPT_FILE}
```

- Worktree path: `.ralph/worktrees/<timestamp>/`
- Branch: `ralph/run-<timestamp>`
- Logs and state are stored inside the worktree

---

## Resume

If a run is interrupted, restart it where it left off:

```bash
./ralph.sh --resume 10 -- claude -p @.ralph/PROMPT.md
```

The current iteration is persisted to `.ralph/iteration.txt` before each agent call.

---

## Stop Signal

By default ralph stops when the agent output contains a line matching:

```
COMPLETE: true
```

Override with `--stop-regex`:

```bash
./ralph.sh --stop-regex '^DONE$' 5 -- my-agent
```

Or set the `STOP_REGEX` environment variable.

---

## Run Summary

After each iteration ralph prints `git diff --stat HEAD` (when inside a Git repo) so you can see exactly what the agent changed:

```
📊 Änderungen seit letztem Commit (git diff --stat HEAD):
 src/api.js | 42 ++++++++++++++++++++++++--
 1 file changed, 40 insertions(+), 2 deletions(-)
```

---

## Signals

| Signal | Behaviour |
|---|---|
| `Ctrl+C` (SIGINT) | Exits with code 130, prints path to last agent output |

---

## Environment Variables

| Variable | Description |
|---|---|
| `RALPH_DELAY` | Default delay between iterations (overridden by `--delay`) |
| `STOP_REGEX` | Default stop regex (overridden by `--stop-regex`) |

---

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Stop condition matched – agent signalled completion |
| `2` | Max iterations reached without stop condition |
| `130` | Interrupted by SIGINT |

---

## File Layout

```
ralph.sh              # The runner
PROMPT_TEMPLATE.md    # Template with {{GOAL}} and {{STACK}} placeholders
.ralph/
  PROMPT.md           # Generated prompt (from --goal/--stack)
  iteration.txt       # Current iteration (for --resume)
  last-output.txt     # Last agent output
  ralph.log           # Full run log
  worktrees/          # Isolated worktrees (--worktree)
```

---

## Examples

See the [`examples/`](examples/) directory:

- [`basic.sh`](examples/basic.sh) – Simple run with a fixed iteration count
- [`with-prompt.sh`](examples/with-prompt.sh) – Using `--goal` and `--stack` for auto-generated prompts
- [`with-worktree.sh`](examples/with-worktree.sh) – Isolated Git worktree run

---

## License

MIT
