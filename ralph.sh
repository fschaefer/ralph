#!/usr/bin/env bash
set -euo pipefail

# Minimal Ralph loop runner
#
# Usage:
#   ./ralph.sh [iterations] -- <agent-command...>
#
# Example:
#   ./ralph.sh 3 -- claude -p "Just output OK"

usage() {
  cat <<'EOF'
Usage:
  ./ralph.sh [options] [iterations] -- <agent-command...>

Options:
  --delay <s>              Pause between iterations in seconds (default: 2, or $RALPH_DELAY)
  --max-iterations <n>     Maximum number of iterations (alias for positional argument)
  --timeout <s>            Per-iteration timeout in seconds; kills agent after <s>s (0 = disabled)
  --stop-regex <pattern>   Regex that triggers a successful stop (default: $STOP_REGEX or ^COMPLETE:...)
  --action-inbox           Pause loop when agent outputs "ACTION_REQUIRED: <msg>";
                           wait for user input and write it to .ralph/inbox-response.txt
  --inbox-timeout <s>      Timeout for user input in seconds (0 = unlimited, default: 0)
  --monitor                Tail .ralph/ralph.log in real-time (open in a second terminal)
  --quiet, -q              Suppress config header and iteration banners
  --dry-run                Print configuration and exit without running the agent
  --resume                 Resume from last saved iteration (.ralph/iteration.txt)
  --worktree               Create an isolated Git worktree for this run (branch: ralph/run-<ts>)
  --goal <text>            Project goal (fills {{GOAL}} in prompt template → .ralph/PROMPT.md)
  --stack <text>           Tech stack (fills {{STACK}} in prompt template → .ralph/PROMPT.md)
  --prompt-file <path>     Use a ready-made prompt file directly (overrides --goal/--stack)
  --spec <name>            Load .ralph/specs/<name>.md as prompt;
                           use {SPEC_FILE} in the agent command as a placeholder
  --extend-spec <name>     Resume a completed project: appends a new task to tasks.md
                           referencing .ralph/specs/<name>.md so the agent picks it up
  -v, --version            Print version number and exit

Prompt integration:
  With --goal and --stack the prompt template is filled and saved as .ralph/PROMPT.md.
  The template is embedded in the script; an external PROMPT_TEMPLATE.md in the project
  directory takes priority over the built-in one.
  Use {PROMPT_FILE} anywhere in your agent command as a placeholder for the path:
    ./ralph.sh --goal "Build a REST API" --stack "Node.js" 5 -- claude -p @{PROMPT_FILE}

Examples:
  ./ralph.sh 3 -- claude -p "Just output OK"
  ./ralph.sh --max-iterations 3 -- claude -p "Just output OK"
  ./ralph.sh --delay 5 3 -- claude -p "Just output OK"
  ./ralph.sh --stop-regex '^DONE$' 3 -- claude -p "Just output OK"
  ./ralph.sh --dry-run 3 -- claude -p "Just output OK"
  ./ralph.sh --resume 3 -- claude -p @{PROMPT_FILE}
  ./ralph.sh --worktree 5 -- claude -p @{PROMPT_FILE}
  ./ralph.sh --goal "Build a REST API" --stack "Node.js, Express" 5 -- claude -p @{PROMPT_FILE}
  ./ralph.sh --timeout 120 5 -- claude -p @{PROMPT_FILE}
  ./ralph.sh --action-inbox 5 -- claude -p @{PROMPT_FILE}
  ./ralph.sh --action-inbox --inbox-timeout 120 5 -- claude -p @{PROMPT_FILE}
  ./ralph.sh --spec myfeature 5 -- claude -p @{SPEC_FILE}
  ./ralph.sh --monitor        # in a second terminal while a run is active

Note:
  By default the loop stops when the agent output contains a line matching:
    COMPLETE: true
  Customise via --stop-regex or the STOP_REGEX environment variable.
EOF
}

ITERATIONS="5"
DELAY="${RALPH_DELAY:-2}"
TIMEOUT=0
DRY_RUN=0
RESUME=0
WORKTREE=0
ACTION_INBOX=0
INBOX_TIMEOUT=0
MONITOR=0
QUIET=0
STOP_REGEX_ARG=""
GOAL=""
STACK=""
PROMPT_FILE_OVERRIDE=""
SPEC_NAME=""
EXTEND_SPEC_NAME=""

# Show help when called without any arguments.
if [[ $# -eq 0 ]]; then
  usage
  exit 0
fi

# Parse named flags and optional positional iterations before '--'
while [[ $# -gt 0 && "${1:-}" != "--" ]]; do
  case "$1" in
    -h|--help)
      usage; exit 0
      ;;
    -v|--version)
      echo "ralph 2.0.0"
      exit 0
      ;;
    --monitor)
      MONITOR=1; shift
      ;;
    -q|--quiet)
      QUIET=1; shift
      ;;
    --delay)
      [[ -z "${2:-}" ]] && { echo "Error: --delay requires a value."; exit 1; }
      DELAY="$2"; shift 2
      ;;
    --delay=*)
      DELAY="${1#*=}"; shift
      ;;
    --max-iterations)
      [[ -z "${2:-}" ]] && { echo "Error: --max-iterations requires a value."; exit 1; }
      ITERATIONS="$2"; shift 2
      ;;
    --max-iterations=*)
      ITERATIONS="${1#*=}"; shift
      ;;
    --timeout)
      [[ -z "${2:-}" ]] && { echo "Error: --timeout requires a value."; exit 1; }
      TIMEOUT="$2"; shift 2
      ;;
    --timeout=*)
      TIMEOUT="${1#*=}"; shift
      ;;
    --stop-regex)
      [[ -z "${2:-}" ]] && { echo "Error: --stop-regex requires a value."; exit 1; }
      STOP_REGEX_ARG="$2"; shift 2
      ;;
    --stop-regex=*)
      STOP_REGEX_ARG="${1#*=}"; shift
      ;;
    --dry-run)
      DRY_RUN=1; shift
      ;;
    --resume)
      RESUME=1; shift
      ;;
    --worktree)
      WORKTREE=1; shift
      ;;
    --action-inbox)
      ACTION_INBOX=1; shift
      ;;
    --inbox-timeout)
      [[ -z "${2:-}" ]] && { echo "Error: --inbox-timeout requires a value."; exit 1; }
      INBOX_TIMEOUT="$2"; shift 2
      ;;
    --inbox-timeout=*)
      INBOX_TIMEOUT="${1#*=}"; shift
      ;;
    --goal)
      [[ -z "${2:-}" ]] && { echo "Error: --goal requires a value."; exit 1; }
      GOAL="$2"; shift 2
      ;;
    --goal=*)
      GOAL="${1#*=}"; shift
      ;;
    --stack)
      [[ -z "${2:-}" ]] && { echo "Error: --stack requires a value."; exit 1; }
      STACK="$2"; shift 2
      ;;
    --stack=*)
      STACK="${1#*=}"; shift
      ;;
    --prompt-file)
      [[ -z "${2:-}" ]] && { echo "Error: --prompt-file requires a value."; exit 1; }
      PROMPT_FILE_OVERRIDE="$2"; shift 2
      ;;
    --prompt-file=*)
      PROMPT_FILE_OVERRIDE="${1#*=}"; shift
      ;;
    --spec)
      [[ -z "${2:-}" ]] && { echo "Error: --spec requires a value."; exit 1; }
      SPEC_NAME="$2"; shift 2
      ;;
    --spec=*)
      SPEC_NAME="${1#*=}"; shift
      ;;
    --extend-spec)
      [[ -z "${2:-}" ]] && { echo "Error: --extend-spec requires a value."; exit 1; }
      EXTEND_SPEC_NAME="$2"; shift 2
      ;;
    --extend-spec=*)
      EXTEND_SPEC_NAME="${1#*=}"; shift
      ;;
    *)
      ITERATIONS="$1"; shift; break
      ;;
  esac
done

# Monitor mode: tail the log without needing an agent command
if [[ $MONITOR -eq 1 ]]; then
  RALPH_DIR=".ralph"
  LOG_FILE="$RALPH_DIR/ralph.log"
  ITERATION_FILE="$RALPH_DIR/iteration.txt"
  if [[ ! -f "$LOG_FILE" ]]; then
    echo "Error: No log file found: $LOG_FILE"
    echo "Start a ralph run first before using --monitor."
    exit 1
  fi
  CURRENT_ITER="?"
  if [[ -f "$ITERATION_FILE" ]]; then
    CURRENT_ITER="$(< "$ITERATION_FILE")"
  fi
  echo "============================================================"
  echo "📡 Ralph Monitor – Live Log"
  printf '  %-18s %s\n' "Log file:" "$LOG_FILE"
  printf '  %-18s %s\n' "Current iteration:" "$CURRENT_ITER"
  echo "============================================================"
  echo "(Press Ctrl+C to stop)"
  echo ""
  tail -n 50 -f "$LOG_FILE"
  exit 0
fi

if [[ "${1:-}" != "--" ]]; then
  echo "Error: '--' separator is missing."
  usage
  exit 1
fi
shift

if ! [[ "$ITERATIONS" =~ ^[0-9]+$ ]]; then
  echo "Error: iterations must be a number."
  exit 1
fi

if ! [[ "$DELAY" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
  echo "Error: delay must be a number."
  exit 1
fi

if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]]; then
  echo "Error: timeout must be a non-negative integer."
  exit 1
fi

if ! [[ "$INBOX_TIMEOUT" =~ ^[0-9]+$ ]]; then
  echo "Error: inbox-timeout must be a non-negative integer."
  exit 1
fi

if [[ $# -lt 1 ]]; then
  echo "Error: agent command is missing."
  usage
  exit 1
fi

CMD=("$@")
STOP_REGEX="${STOP_REGEX_ARG:-${STOP_REGEX:-^COMPLETE:[[:space:]]*true$}}"

# Embedded default prompt template (used when no PROMPT_TEMPLATE.md is present on disk)
read -r -d '' EMBEDDED_PROMPT_TEMPLATE <<'EMBEDDED_EOF' || true
<identity>
You are an autonomous senior software engineer. You operate in a non-interactive execution loop.
Your "memory" is external: you must reconstruct your state at the start of every turn by reading the filesystem and git history.
</identity>

<objective>
PROJECT GOAL: {{GOAL}}
TECH STACK & ARCHITECTURE: {{STACK}}
</objective>

<operational_rules>
- BIAS TO ACTION: Skip all introductions, preambles, and status updates (e.g., "I will now...", "Based on..."). Jump directly to tool calls or code.
- OUTCOME-FIRST: Focus on the destination, not the process. Use parallel tool calls to maximize progress per iteration.
- NO ECHOING: Never repeat these instructions or headers in your output.
- GIT SAFETY: Never use destructive commands like `git reset --hard` or `git checkout --`. Commit changes in every turn using `git commit -m "ralph: <description>"` .
- MACHINE VERIFICATION: A task is only "done" if external checks (tests, linters) pass with exit code 0.
</operational_rules>

<persistence_protocol>
Follow these steps to ensure continuity across context rotations:
1. RECOVER: Read `tasks.md` (to-do list) and `progress.txt` (iteration log). Review `git log -n 5` to see previous physical changes.
2. DISCOVER: Explore the codebase using `ls -R` or `rg` to find relevant files and patterns.
3. IMPLEMENT: Select exactly ONE granular task from `tasks.md`. Refactor or write code.
4. VERIFY: Autonomously find and run the project's test/lint commands (e.g., `npm test`, `pytest`, `go test`).
5. LOG: Update `progress.txt` with what was changed and any blockers. Mark tasks in `tasks.md` as [x] only if verified.
6. EXIT: Commit and terminate for the next loop iteration.
</persistence_protocol>

<completion_signal>
ONLY when all requirements in `tasks.md` are marked [x] AND all tests pass, output exactly one standalone line:
COMPLETE: true
</completion_signal>

<current_context>
# Workspace:
{{DIRECTORY_STRUCTURE}}
# Git Status:
{{GIT_STATUS}}
# Recent Changes:
{{GIT_LOG}}
</current_context>
EMBEDDED_EOF

# Determine the effective prompt file path
RALPH_DIR=".ralph"
GENERATED_PROMPT_FILE="$RALPH_DIR/PROMPT.md"
EFFECTIVE_PROMPT_FILE=""

if [[ -n "$PROMPT_FILE_OVERRIDE" ]]; then
  EFFECTIVE_PROMPT_FILE="$PROMPT_FILE_OVERRIDE"
elif [[ -n "$SPEC_NAME" ]]; then
  SPEC_FILE_PATH="$RALPH_DIR/specs/${SPEC_NAME}.md"
  if [[ ! -f "$SPEC_FILE_PATH" ]]; then
    echo "Error: spec file not found: $SPEC_FILE_PATH"
    exit 1
  fi
  EFFECTIVE_PROMPT_FILE="$SPEC_FILE_PATH"
elif [[ -n "$GOAL" || -n "$STACK" ]]; then
  TEMPLATE_FILE="PROMPT_TEMPLATE.md"
  mkdir -p "$RALPH_DIR"
  DIR_STRUCTURE="$(find . -maxdepth 3 -not -path './.git/*' 2>/dev/null || true)"
  GIT_STATUS_OUT="$(git status --short 2>/dev/null || true)"
  GIT_LOG_OUT="$(git log --oneline -n 5 2>/dev/null || true)"
  if [[ -f "$TEMPLATE_FILE" ]]; then
    sed \
      -e "s|{{GOAL}}|${GOAL}|g" \
      -e "s|{{STACK}}|${STACK}|g" \
      -e "s|{{DIRECTORY_STRUCTURE}}|${DIR_STRUCTURE}|g" \
      -e "s|{{GIT_STATUS}}|${GIT_STATUS_OUT}|g" \
      -e "s|{{GIT_LOG}}|${GIT_LOG_OUT}|g" \
      "$TEMPLATE_FILE" > "$GENERATED_PROMPT_FILE"
  else
    printf '%s' "$EMBEDDED_PROMPT_TEMPLATE" \
      | sed \
          -e "s|{{GOAL}}|${GOAL}|g" \
          -e "s|{{STACK}}|${STACK}|g" \
          -e "s|{{DIRECTORY_STRUCTURE}}|${DIR_STRUCTURE}|g" \
          -e "s|{{GIT_STATUS}}|${GIT_STATUS_OUT}|g" \
          -e "s|{{GIT_LOG}}|${GIT_LOG_OUT}|g" \
      > "$GENERATED_PROMPT_FILE"
  fi
  EFFECTIVE_PROMPT_FILE="$GENERATED_PROMPT_FILE"
fi

# Replace {PROMPT_FILE} and {SPEC_FILE} placeholders in CMD args
if [[ -n "$EFFECTIVE_PROMPT_FILE" ]]; then
  for i in "${!CMD[@]}"; do
    CMD[$i]="${CMD[$i]//\{PROMPT_FILE\}/$EFFECTIVE_PROMPT_FILE}"
    CMD[$i]="${CMD[$i]//\{SPEC_FILE\}/$EFFECTIVE_PROMPT_FILE}"
  done
fi

if [[ $DRY_RUN -eq 1 ]]; then
  echo "🔍 Dry-run – configuration (no command will be executed):"
  printf '  %-14s %s\n' "Iterations:" "$ITERATIONS"
  printf '  %-14s %s\n' "Delay:" "${DELAY}s"
  printf '  %-14s %s\n' "Timeout:" "$( [[ $TIMEOUT -gt 0 ]] && echo "${TIMEOUT}s" || echo 'disabled' )"
  printf '  %-14s %s\n' "Stop regex:" "$STOP_REGEX"
  printf '  %-14s %s\n' "Resume:" "$( [[ $RESUME -eq 1 ]] && echo 'yes' || echo 'no' )"
  printf '  %-14s %s\n' "Worktree:" "$( [[ $WORKTREE -eq 1 ]] && echo 'yes' || echo 'no' )"
  printf '  %-14s %s\n' "Action inbox:" "$( [[ $ACTION_INBOX -eq 1 ]] && echo "yes (timeout: $( [[ $INBOX_TIMEOUT -gt 0 ]] && echo "${INBOX_TIMEOUT}s" || echo 'unlimited' ))" || echo 'no' )"
  if [[ -n "$EXTEND_SPEC_NAME" ]]; then
    printf '  %-14s %s\n' "Extend spec:" ".ralph/specs/${EXTEND_SPEC_NAME}.md"
  fi
  if [[ -n "$EFFECTIVE_PROMPT_FILE" ]]; then
    if [[ -n "$PROMPT_FILE_OVERRIDE" ]]; then
      printf '  %-14s %s\n' "Prompt file:" "$EFFECTIVE_PROMPT_FILE (--prompt-file)"
    elif [[ -n "$SPEC_NAME" ]]; then
      printf '  %-14s %s\n' "Prompt file:" "$EFFECTIVE_PROMPT_FILE (--spec $SPEC_NAME)"
    elif [[ -f "PROMPT_TEMPLATE.md" ]]; then
      printf '  %-14s %s\n' "Prompt file:" "$EFFECTIVE_PROMPT_FILE (from PROMPT_TEMPLATE.md)"
    else
      printf '  %-14s %s\n' "Prompt file:" "$EFFECTIVE_PROMPT_FILE (built-in template)"
    fi
  fi
  printf '  %-14s %s\n' "Command:" "${CMD[*]}"
  exit 0
fi

ITER_STATUSES=()
RUN_START_TS=0

print_summary() {
  local outcome="${1:-unknown}"
  local elapsed=$(( $(date +%s) - RUN_START_TS ))
  local mins=$(( elapsed / 60 ))
  local secs=$(( elapsed % 60 ))
  echo ""
  echo "============================================================"
  echo "📋 Run Summary"
  echo "============================================================"
  printf '  %-20s %dm %02ds\n' "Total time:" "$mins" "$secs"
  if [[ ${#ITER_STATUSES[@]} -gt 0 ]]; then
    echo ""
    printf '  %-6s  %-6s  %s\n' "Iter." "Exit" "Status"
    printf '  %-6s  %-6s  %s\n' "------" "------" "------"
    local entry iter rest code note
    for entry in "${ITER_STATUSES[@]}"; do
      iter="${entry%%:*}"
      rest="${entry#*:}"
      code="${rest%%:*}"
      note="${rest#*:}"
      printf '  %-6s  %-6s  %s\n' "$iter" "$code" "$note"
    done
  fi
  echo ""
  printf '  %-20s %s\n' "Outcome:" "$outcome"
  echo "============================================================"
}

trap 'echo; print_summary "⚠️  Interrupted (SIGINT)"; echo "Last output in $LAST_OUTPUT_FILE"; exit 130' INT

mkdir -p "$RALPH_DIR"
LOG_FILE="$RALPH_DIR/ralph.log"
LAST_OUTPUT_FILE="$RALPH_DIR/last-output.txt"
ITERATION_FILE="$RALPH_DIR/iteration.txt"
INBOX_RESPONSE_FILE="$RALPH_DIR/inbox-response.txt"
mkdir -p "$RALPH_DIR"

# Git Worktree Isolation
WORKTREE_PATH=""
WORKTREE_BRANCH=""
if [[ $WORKTREE -eq 1 ]]; then
  if ! git rev-parse --is-inside-work-tree &>/dev/null; then
    echo "Error: --worktree requires a Git repository."
    exit 1
  fi
  RUN_TS="$(date '+%Y%m%d-%H%M%S')"
  WORKTREE_BRANCH="ralph/run-${RUN_TS}"
  WORKTREE_PATH="$(git rev-parse --show-toplevel)/.ralph/worktrees/${RUN_TS}"
  mkdir -p "$(dirname "$WORKTREE_PATH")"
  git worktree add -b "$WORKTREE_BRANCH" "$WORKTREE_PATH" HEAD
  echo "🌿 Worktree created: $WORKTREE_PATH (branch: $WORKTREE_BRANCH)"
  # Copy generated prompt file into worktree if needed
  if [[ -n "$EFFECTIVE_PROMPT_FILE" && -f "$EFFECTIVE_PROMPT_FILE" ]]; then
    WT_RALPH_DIR="$WORKTREE_PATH/.ralph"
    mkdir -p "$WT_RALPH_DIR"
    cp "$EFFECTIVE_PROMPT_FILE" "$WT_RALPH_DIR/PROMPT.md"
    # Update CMD to use the worktree-local prompt path
    WT_PROMPT="$WT_RALPH_DIR/PROMPT.md"
    for i in "${!CMD[@]}"; do
      CMD[$i]="${CMD[$i]//$EFFECTIVE_PROMPT_FILE/$WT_PROMPT}"
    done
    EFFECTIVE_PROMPT_FILE="$WT_PROMPT"
  fi
  # Redirect log/output files into worktree
  WT_RALPH_DIR="$WORKTREE_PATH/.ralph"
  mkdir -p "$WT_RALPH_DIR"
  LOG_FILE="$WT_RALPH_DIR/ralph.log"
  LAST_OUTPUT_FILE="$WT_RALPH_DIR/last-output.txt"
  ITERATION_FILE="$WT_RALPH_DIR/iteration.txt"
  INBOX_RESPONSE_FILE="$WT_RALPH_DIR/inbox-response.txt"
  # Change to worktree directory for the agent run
  cd "$WORKTREE_PATH"
fi

i=1
if [[ $RESUME -eq 1 && -f "$ITERATION_FILE" ]]; then
  saved=$(< "$ITERATION_FILE")
  if [[ "$saved" =~ ^[0-9]+$ && $saved -ge 1 && $saved -le $ITERATIONS ]]; then
    i=$saved
    echo "▶️  Resume: starting at iteration $i"
  fi
fi

if [[ $QUIET -eq 0 ]]; then
  printf '%s\n' "--- Ralph Configuration ---"
  printf '  %-18s %s\n' "Iterations:"   "$ITERATIONS"
  printf '  %-18s %s\n' "Delay:"        "${DELAY}s"
  printf '  %-18s %s\n' "Timeout:"      "$( [[ $TIMEOUT -gt 0 ]] && echo "${TIMEOUT}s" || echo 'disabled' )"
  printf '  %-18s %s\n' "Stop regex:"   "$STOP_REGEX"
  printf '  %-18s %s\n' "Resume:"       "$( [[ $RESUME -eq 1 ]] && echo 'yes' || echo 'no' )"
  printf '  %-18s %s\n' "Worktree:"     "$( [[ $WORKTREE -eq 1 ]] && echo "$WORKTREE_PATH" || echo 'no' )"
  printf '  %-18s %s\n' "Action inbox:" "$( [[ $ACTION_INBOX -eq 1 ]] && echo "yes (timeout: $( [[ $INBOX_TIMEOUT -gt 0 ]] && echo "${INBOX_TIMEOUT}s" || echo 'unlimited' ))" || echo 'no' )"
  if [[ -n "$EXTEND_SPEC_NAME" ]]; then
    printf '  %-18s %s\n' "Extend spec:" ".ralph/specs/${EXTEND_SPEC_NAME}.md"
  fi
  printf '  %-18s %s\n' "Log file:"     "$LOG_FILE"
  if [[ -n "$EFFECTIVE_PROMPT_FILE" ]]; then
    if [[ -n "$PROMPT_FILE_OVERRIDE" ]]; then
      printf '  %-18s %s\n' "Prompt file:" "$EFFECTIVE_PROMPT_FILE (--prompt-file)"
    elif [[ -n "$SPEC_NAME" ]]; then
      printf '  %-18s %s\n' "Prompt file:" "$EFFECTIVE_PROMPT_FILE (--spec $SPEC_NAME)"
    elif [[ -f "PROMPT_TEMPLATE.md" ]]; then
      printf '  %-18s %s\n' "Prompt file:" "$EFFECTIVE_PROMPT_FILE (from PROMPT_TEMPLATE.md)"
    else
      printf '  %-18s %s\n' "Prompt file:" "$EFFECTIVE_PROMPT_FILE (built-in template)"
    fi
  fi
  printf '  Command:           '
  printf '%s ' "${CMD[@]}"; echo
  echo ""
fi

RUN_START_TS=$(date +%s)

# Extend-spec: inject new tasks into tasks.md so the agent can pick them up
if [[ -n "$EXTEND_SPEC_NAME" ]]; then
  EXTEND_SPEC_FILE="${RALPH_DIR}/specs/${EXTEND_SPEC_NAME}.md"
  if [[ ! -f "$EXTEND_SPEC_FILE" ]]; then
    echo "Error: extend-spec file not found: $EXTEND_SPEC_FILE"
    exit 1
  fi
  TASKS_FILE="tasks.md"
  PROGRESS_FILE="progress.txt"
  EXTEND_TIMESTAMP="$(date '+%Y-%m-%d %H:%M:%S')"
  printf '\n## Extension: %s (%s)\n\n- [ ] Implement new requirements from .ralph/specs/%s.md (read the file for details)\n' \
    "$EXTEND_SPEC_NAME" "$EXTEND_TIMESTAMP" "$EXTEND_SPEC_NAME" >> "$TASKS_FILE"
  printf '[%s] Extension added via --extend-spec %s\n' \
    "$EXTEND_TIMESTAMP" "$EXTEND_SPEC_NAME" >> "$PROGRESS_FILE"
  echo "📎 Extension injected into $TASKS_FILE from $EXTEND_SPEC_FILE"
fi

while [[ $i -le $ITERATIONS ]]; do
  echo "$i" > "$ITERATION_FILE"

  if [[ $QUIET -eq 0 ]]; then
    echo "============================================================"
    echo "Iteration $i/$ITERATIONS"
    echo "============================================================"
  fi

  set +e
  if [[ $TIMEOUT -gt 0 ]]; then
    if command -v timeout >/dev/null 2>&1; then
      timeout "$TIMEOUT" "${CMD[@]}" 2>&1 | tee "$LAST_OUTPUT_FILE"
      EXIT_CODE=${PIPESTATUS[0]}
    else
      echo "⚠️  Warning: 'timeout' command not found; running without timeout" >&2
      "${CMD[@]}" 2>&1 | tee "$LAST_OUTPUT_FILE"
      EXIT_CODE=${PIPESTATUS[0]}
    fi
  else
    "${CMD[@]}" 2>&1 | tee "$LAST_OUTPUT_FILE"
    EXIT_CODE=${PIPESTATUS[0]}
  fi
  set -e

  {
    echo "[$(date '+%F %T')] Iteration $i exit=$EXIT_CODE"
    cat "$LAST_OUTPUT_FILE"
    echo
  } >> "$LOG_FILE"

  if git rev-parse --is-inside-work-tree &>/dev/null 2>&1; then
    DIFF_STAT="$(git diff --stat HEAD 2>/dev/null)"
    if [[ -n "$DIFF_STAT" ]]; then
      echo ""
      echo "📊 Changes since last commit (git diff --stat HEAD):"
      echo "$DIFF_STAT"
    fi
  fi

  STOP_MATCHED=0
  grep -Eiq "$STOP_REGEX" "$LAST_OUTPUT_FILE" && STOP_MATCHED=1 || true

  if [[ $STOP_MATCHED -eq 1 ]]; then
    _ITER_NOTE="✓ stop"
  elif [[ $TIMEOUT -gt 0 && $EXIT_CODE -eq 124 ]]; then
    _ITER_NOTE="⏱ timeout"
  elif [[ $EXIT_CODE -ne 0 ]]; then
    _ITER_NOTE="✗ error"
  else
    _ITER_NOTE="→ continue"
  fi
  ITER_STATUSES+=("$i:$EXIT_CODE:$_ITER_NOTE")

  if [[ $STOP_MATCHED -eq 1 ]]; then
    echo "✅ Stop condition matched in iteration $i"
    print_summary "✅ Stop condition matched (iteration $i)"
    exit 0
  fi

  if [[ $ACTION_INBOX -eq 1 ]]; then
    ACTION_LINE="$(grep -Eo 'ACTION_REQUIRED:[[:space:]]*.*' "$LAST_OUTPUT_FILE" | head -1 || true)"
    if [[ -n "$ACTION_LINE" ]]; then
      ACTION_MSG="${ACTION_LINE#ACTION_REQUIRED:}"
      ACTION_MSG="${ACTION_MSG#"${ACTION_MSG%%[![:space:]]*}"}"
      echo ""
      echo "📬 Action Inbox – agent is waiting for input:"
      echo "   $ACTION_MSG"
      echo ""
      if [[ $INBOX_TIMEOUT -gt 0 ]]; then
        read -rp "Your reply (${INBOX_TIMEOUT}s timeout): " -t "$INBOX_TIMEOUT" USER_RESPONSE || {
          echo ""
          echo "⏱️ Timeout – no input received. Continuing without a reply."
          USER_RESPONSE=""
        }
      else
        read -rp "Your reply: " USER_RESPONSE
      fi
      echo "$USER_RESPONSE" > "$INBOX_RESPONSE_FILE"
      echo "✅ Reply saved to $INBOX_RESPONSE_FILE"
    fi
  fi

  ((i++))
  sleep "$DELAY"
done

echo "⚠️ Max iterations reached."
print_summary "⚠️  Max iterations ($ITERATIONS) reached"
exit 2
