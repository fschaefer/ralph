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
  ./ralph.sh --resume 3 -- claude -p @.ralph/PROMPT.md
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

# Parse named flags and optional positional iterations before '--'
while [[ $# -gt 0 && "${1:-}" != "--" ]]; do
  case "$1" in
    -h|--help)
      usage; exit 0
      ;;
    -v|--version)
      echo "ralph 1.0.0"
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
SYSTEM DIRECTIVE: AUTONOMOUS RALPH LOOP AGENT
You are an autonomous software engineering agent driven by an external Bash loop. You have no memory between iterations. Your entire knowledge of the project lives exclusively in the filesystem.

1. PROJECT GOAL & SPECIFICATION

- You are building the following project:
{{GOAL}}

- Tech stack & architecture rules:
{{STACK}}

2. STRICT WORKFLOW (ALWAYS FOLLOW IN ORDER!)
Follow these steps exactly in the order given. Do not skip any step.

STEP 1: Orientation (State Recovery)

Read tasks.md (the to-do list) and progress.txt (the log left by previous iterations).

If these files do not exist: this is iteration 1. Create the basic project structure. Create tasks.md with a very granular checklist based on the project goal. Create an empty progress.txt.

STEP 2: Task selection

Identify the next single, logically isolated task in tasks.md that is not yet done.

Never tackle multiple complex things at once.

STEP 3: Implementation

Implement the selected task. Write or refactor the relevant code.

STEP 4: Backpressure & Verification (EXTREMELY IMPORTANT)
Never assume your code works. You must use external validation.

Analyse the project structure autonomously (e.g. read files like package.json, Makefile, Cargo.toml or explore the directory tree) to find out which linter, type-check, and test commands this specific project uses.

Run the identified check commands (e.g. npm test, tsc --noEmit, pytest) via your terminal tool.
If a test or linter fails, analyse the error and fix the code. If you are stuck, document it in step 5 and terminate for the next iteration.

STEP 5: Update memory (Memory Injection)

Append a short entry to progress.txt: which task was worked on, which files were changed, any unresolved errors. (Be brief!)

Mark the task in tasks.md as done (e.g. [x]) only when the code has been written and successfully verified by the commands in step 4 with no errors.

STEP 6: Git commit

Run via terminal: git add . and then git commit with a concise, descriptive message
that summarises what was implemented in this iteration. Use the format:
  ralph: <short description of the task>
Examples:
  git commit -m "ralph: add user authentication endpoint"
  git commit -m "ralph: fix input validation in registration form"
  git commit -m "ralph: implement pagination for product list"
Note: make sure .ralph/ is listed in .gitignore so the runner's state files are not committed.

STEP 7: Termination

Scenario A (there are still open tasks or errors): End your output with a short summary. The external loop will restart you for the next task.

Scenario B (ALL tasks are done AND verified): Only when absolutely all requirements from tasks.md have been completed and all external checks pass without errors, output exactly the following string as a standalone line:

COMPLETE: true
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
  if [[ -f "$TEMPLATE_FILE" ]]; then
    sed \
      -e "s|{{GOAL}}|${GOAL}|g" \
      -e "s|{{STACK}}|${STACK}|g" \
      "$TEMPLATE_FILE" > "$GENERATED_PROMPT_FILE"
  else
    printf '%s' "$EMBEDDED_PROMPT_TEMPLATE" \
      | sed \
          -e "s|{{GOAL}}|${GOAL}|g" \
          -e "s|{{STACK}}|${STACK}|g" \
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
  echo "- Iterations:   $ITERATIONS"
  echo "- Delay:        ${DELAY}s"
  echo "- Timeout:      $( [[ $TIMEOUT -gt 0 ]] && echo "${TIMEOUT}s" || echo 'disabled' )"
  echo "- Stop regex:   $STOP_REGEX"
  echo "- Resume:       $( [[ $RESUME -eq 1 ]] && echo 'yes' || echo 'no' )"
  echo "- Worktree:     $( [[ $WORKTREE -eq 1 ]] && echo 'yes' || echo 'no' )"
  echo "- Action inbox: $( [[ $ACTION_INBOX -eq 1 ]] && echo "yes (timeout: $( [[ $INBOX_TIMEOUT -gt 0 ]] && echo "${INBOX_TIMEOUT}s" || echo 'unlimited' ))" || echo 'no' )"
  if [[ -n "$EFFECTIVE_PROMPT_FILE" ]]; then
    if [[ -n "$PROMPT_FILE_OVERRIDE" ]]; then
      echo "- Prompt file:  $EFFECTIVE_PROMPT_FILE (--prompt-file)"
    elif [[ -n "$SPEC_NAME" ]]; then
      echo "- Prompt file:  $EFFECTIVE_PROMPT_FILE (--spec $SPEC_NAME)"
    elif [[ -f "PROMPT_TEMPLATE.md" ]]; then
      echo "- Prompt file:  $EFFECTIVE_PROMPT_FILE (from PROMPT_TEMPLATE.md)"
    else
      echo "- Prompt file:  $EFFECTIVE_PROMPT_FILE (built-in template)"
    fi
  fi
  printf '%s ' "- Command:" "${CMD[@]}"; echo
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
