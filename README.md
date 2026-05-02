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
| `--timeout <s>` | `0` (off) | Per-iteration timeout; kills agent after `<s>` seconds |
| `--stop-regex <pattern>` | `^COMPLETE:\s*true$` (or `$STOP_REGEX`) | Regex that triggers a successful stop |
| `--dry-run` | off | Print configuration and exit without running |
| `--resume` | off | Resume from last saved iteration (`.ralph/iteration.txt`) |
| `--worktree` | off | Run the agent inside an isolated Git worktree |
| `--action-inbox` | off | Pause when agent outputs `ACTION_REQUIRED: <msg>`; wait for user input |
| `--inbox-timeout <s>` | `0` (unlimited) | Timeout for user input prompt (requires `--action-inbox`) |
| `--monitor` | off | Tail `.ralph/ralph.log` in real-time (open in a second terminal) |
| `--goal <text>` | – | Project goal – fills `{{GOAL}}` in `PROMPT_TEMPLATE.md` |
| `--stack <text>` | – | Tech stack – fills `{{STACK}}` in `PROMPT_TEMPLATE.md` |
| `--prompt-file <path>` | – | Use a ready-made prompt file (overrides `--goal`/`--stack`) |
| `--spec <name>` | – | Load `.ralph/specs/<name>.md` as prompt; use `{SPEC_FILE}` in the agent command |
| `-h`, `--help` | – | Show help and exit |

---

## PROMPT Integration

ralph can auto-generate a structured agent prompt by substituting two placeholders:

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

**No external file required.** ralph has a complete autonomous-agent prompt template built in. It is used automatically when `--goal` or `--stack` is provided and no `PROMPT_TEMPLATE.md` is found in the working directory.

To customise the template, place a `PROMPT_TEMPLATE.md` in your project root — it takes priority over the built-in one.

You can also provide a hand-crafted prompt file directly:

```bash
./ralph.sh --prompt-file my-prompt.md 10 -- claude -p @{PROMPT_FILE}
```

### Minimal prompt for the agent

The agent only needs to output `COMPLETE: true` on its own line when all tasks are done. Everything else – task tracking, progress log, git commits – is managed by the agent itself via `tasks.md` and `progress.txt` (as defined in the built-in template).

---

## Multiple Specs

Store reusable task-specific prompts in `.ralph/specs/` and load them with `--spec <name>`:

```bash
# Create specs directory and a spec
mkdir -p .ralph/specs
cp my-feature-prompt.md .ralph/specs/auth.md

# Run with the spec
./ralph.sh --spec auth 10 -- claude -p @{SPEC_FILE}
```

- `--spec auth` loads `.ralph/specs/auth.md`
- Use `{SPEC_FILE}` anywhere in your agent command as a placeholder for the spec file path
- The spec file is used as-is (no template substitution); for template substitution use `--goal`/`--stack` instead

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

## Action Inbox

Enable `--action-inbox` to let the agent pause the loop and ask you a question mid-run.

When the agent outputs a line starting with `ACTION_REQUIRED:`, ralph stops the loop, shows the message, and waits for your typed response. The response is written to `.ralph/inbox-response.txt` so the agent can read it on the next iteration.

```bash
./ralph.sh --action-inbox 10 -- claude -p @.ralph/PROMPT.md
```

**Agent side** – the agent emits the signal like this:

```
ACTION_REQUIRED: Which database should I use – SQLite or PostgreSQL?
```

**User side** – ralph pauses and prompts:

```
📬 Action Inbox – Agent wartet auf Eingabe:
   Which database should I use – SQLite or PostgreSQL?

Deine Antwort: SQLite please
✅ Antwort gespeichert in .ralph/inbox-response.txt
```

On the next iteration the agent reads `.ralph/inbox-response.txt` and continues.

### Optional timeout

Use `--inbox-timeout <s>` to automatically continue if no input is received within the given number of seconds. If the timeout fires, an empty response is written and the loop continues.

```bash
./ralph.sh --action-inbox --inbox-timeout 60 10 -- claude -p @.ralph/PROMPT.md
```

---

## Monitor Mode

Watch a running ralph session live from a second terminal:

```bash
# Terminal 1 – start the run
./ralph.sh --goal "Build a REST API" --stack "Node.js" 10 -- claude -p @{PROMPT_FILE}

# Terminal 2 – tail the live log
./ralph.sh --monitor
```

`--monitor` shows the last 50 log lines and then follows `.ralph/ralph.log` in real-time, including the current iteration number. Press `Ctrl+C` to stop monitoring; the run in Terminal 1 is unaffected.

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
ralph.sh              # The runner (includes built-in prompt template)
PROMPT_TEMPLATE.md    # Optional: custom template with {{GOAL}} and {{STACK}} (overrides built-in)
.ralph/
  PROMPT.md           # Generated prompt (from --goal/--stack)
  iteration.txt       # Current iteration (for --resume)
  last-output.txt     # Last agent output
  ralph.log           # Full run log
  inbox-response.txt  # User response written by --action-inbox
  specs/              # Named spec files (for --spec)
    <name>.md
  worktrees/          # Isolated worktrees (--worktree)
```

---

## Examples

See the [`examples/`](examples/) directory:

- [`basic.sh`](examples/basic.sh) – Simple run with a fixed iteration count
- [`with-prompt.sh`](examples/with-prompt.sh) – Using `--goal` and `--stack` for auto-generated prompts
- [`with-worktree.sh`](examples/with-worktree.sh) – Isolated Git worktree run
- [`with-action-inbox.sh`](examples/with-action-inbox.sh) – Interactive Action Inbox pause-and-approve
- [`monitor.sh`](examples/monitor.sh) – Live log monitoring in a second terminal

---

## License

MIT
