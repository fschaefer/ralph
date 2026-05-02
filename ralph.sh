#!/usr/bin/env bash
set -euo pipefail

# Minimaler Ralph-Loop-Runner
#
# Usage:
#   ./ralph.sh [iterations] -- <agent-command...>
#
# Beispiel:
#   ./ralph.sh 3 -- pi -p "Schreib einfach nur OK"

usage() {
  cat <<'EOF'
Usage:
  ./ralph.sh [options] [iterations] -- <agent-command...>

Optionen:
  --delay <s>              Pause zwischen Iterationen in Sekunden (Standard: 2, oder RALPH_DELAY)
  --max-iterations <n>     Maximale Anzahl Iterationen (Alias für Positionsargument)
  --stop-regex <pattern>   Regex zum Erkennen des Stopp-Signals (Standard: STOP_REGEX oder ^COMPLETE:...)
  --dry-run                Konfiguration ausgeben, ohne den Befehl auszuführen; exit 0
  --resume                 Bei der zuletzt gespeicherten Iteration weitermachen (.ralph/iteration.txt)
  --worktree               Isolierten Git Worktree für diesen Run erstellen (Branch: ralph/run-<ts>)
  --goal <text>            Projektziel (befüllt {{GOAL}} in PROMPT_TEMPLATE.md → .ralph/PROMPT.md)
  --stack <text>           Technologie-Stack (befüllt {{STACK}} in PROMPT_TEMPLATE.md → .ralph/PROMPT.md)
  --prompt-file <path>     Fertigen Prompt direkt übergeben (überschreibt --goal/--stack)
  -v, --version            Versionsnummer ausgeben und beenden

Prompt-Integration:
  Mit --goal und --stack wird PROMPT_TEMPLATE.md befüllt und als .ralph/PROMPT.md gespeichert.
  Im Agent-Kommando kann {PROMPT_FILE} als Platzhalter für den Pfad verwendet werden:
    ./ralph.sh --goal "Build a REST API" --stack "Node.js" 5 -- claude -p @{PROMPT_FILE}

Beispiel:
  ./ralph.sh 3 -- pi -p "Schreib einfach nur OK"
  ./ralph.sh --max-iterations 3 -- pi -p "Schreib einfach nur OK"
  ./ralph.sh --delay 5 3 -- pi -p "Schreib einfach nur OK"
  ./ralph.sh --stop-regex '^DONE$' 3 -- pi -p "Schreib einfach nur OK"
  ./ralph.sh --dry-run 3 -- pi -p "Schreib einfach nur OK"
  ./ralph.sh --resume 3 -- pi -p "Schreib einfach nur OK"
  ./ralph.sh --worktree 5 -- claude -p @{PROMPT_FILE}
  ./ralph.sh --goal "Baue eine REST API" --stack "Node.js, Express" 5 -- claude -p @{PROMPT_FILE}

Hinweis:
  Standardmäßig stoppt der Loop bei einer Zeile wie:
    COMPLETE: true
  Regex anpassbar über --stop-regex oder STOP_REGEX.
EOF
}

ITERATIONS="5"
DELAY="${RALPH_DELAY:-2}"
DRY_RUN=0
RESUME=0
WORKTREE=0
STOP_REGEX_ARG=""
GOAL=""
STACK=""
PROMPT_FILE_OVERRIDE=""

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
    --delay)
      [[ -z "${2:-}" ]] && { echo "Fehler: --delay benötigt einen Wert."; exit 1; }
      DELAY="$2"; shift 2
      ;;
    --delay=*)
      DELAY="${1#*=}"; shift
      ;;
    --max-iterations)
      [[ -z "${2:-}" ]] && { echo "Fehler: --max-iterations benötigt einen Wert."; exit 1; }
      ITERATIONS="$2"; shift 2
      ;;
    --max-iterations=*)
      ITERATIONS="${1#*=}"; shift
      ;;
    --stop-regex)
      [[ -z "${2:-}" ]] && { echo "Fehler: --stop-regex benötigt einen Wert."; exit 1; }
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
    --goal)
      [[ -z "${2:-}" ]] && { echo "Fehler: --goal benötigt einen Wert."; exit 1; }
      GOAL="$2"; shift 2
      ;;
    --goal=*)
      GOAL="${1#*=}"; shift
      ;;
    --stack)
      [[ -z "${2:-}" ]] && { echo "Fehler: --stack benötigt einen Wert."; exit 1; }
      STACK="$2"; shift 2
      ;;
    --stack=*)
      STACK="${1#*=}"; shift
      ;;
    --prompt-file)
      [[ -z "${2:-}" ]] && { echo "Fehler: --prompt-file benötigt einen Wert."; exit 1; }
      PROMPT_FILE_OVERRIDE="$2"; shift 2
      ;;
    --prompt-file=*)
      PROMPT_FILE_OVERRIDE="${1#*=}"; shift
      ;;
    *)
      ITERATIONS="$1"; shift; break
      ;;
  esac
done

if [[ "${1:-}" != "--" ]]; then
  echo "Fehler: '--' als Trenner fehlt."
  usage
  exit 1
fi
shift

if ! [[ "$ITERATIONS" =~ ^[0-9]+$ ]]; then
  echo "Fehler: iterations muss eine Zahl sein."
  exit 1
fi

if ! [[ "$DELAY" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
  echo "Fehler: delay muss eine Zahl sein."
  exit 1
fi

if [[ $# -lt 1 ]]; then
  echo "Fehler: Agent-Kommando fehlt."
  usage
  exit 1
fi

CMD=("$@")
STOP_REGEX="${STOP_REGEX_ARG:-${STOP_REGEX:-^COMPLETE:[[:space:]]*true$}}"

# Determine the effective prompt file path
RALPH_DIR=".ralph"
GENERATED_PROMPT_FILE="$RALPH_DIR/PROMPT.md"
EFFECTIVE_PROMPT_FILE=""

if [[ -n "$PROMPT_FILE_OVERRIDE" ]]; then
  EFFECTIVE_PROMPT_FILE="$PROMPT_FILE_OVERRIDE"
elif [[ -n "$GOAL" || -n "$STACK" ]]; then
  TEMPLATE_FILE="PROMPT_TEMPLATE.md"
  if [[ ! -f "$TEMPLATE_FILE" ]]; then
    echo "Fehler: PROMPT_TEMPLATE.md nicht gefunden. Bitte im Projektverzeichnis ablegen."
    exit 1
  fi
  mkdir -p "$RALPH_DIR"
  sed \
    -e "s|{{GOAL}}|${GOAL}|g" \
    -e "s|{{STACK}}|${STACK}|g" \
    "$TEMPLATE_FILE" > "$GENERATED_PROMPT_FILE"
  EFFECTIVE_PROMPT_FILE="$GENERATED_PROMPT_FILE"
fi

# Replace {PROMPT_FILE} placeholder in CMD args
if [[ -n "$EFFECTIVE_PROMPT_FILE" ]]; then
  for i in "${!CMD[@]}"; do
    CMD[$i]="${CMD[$i]//\{PROMPT_FILE\}/$EFFECTIVE_PROMPT_FILE}"
  done
fi

if [[ $DRY_RUN -eq 1 ]]; then
  echo "🔍 Dry-run – Konfiguration (kein Befehl wird ausgeführt):"
  echo "- Iterationen: $ITERATIONS"
  echo "- Delay:       ${DELAY}s"
  echo "- Stop-Regex:  $STOP_REGEX"
  echo "- Resume:      $( [[ $RESUME -eq 1 ]] && echo 'ja' || echo 'nein' )"
  echo "- Worktree:    $( [[ $WORKTREE -eq 1 ]] && echo 'ja' || echo 'nein' )"
  [[ -n "$EFFECTIVE_PROMPT_FILE" ]] && echo "- Prompt-Datei: $EFFECTIVE_PROMPT_FILE"
  printf '%s ' "- Kommando:" "${CMD[@]}"; echo
  exit 0
fi

trap 'echo; echo "⚠️ Unterbrochen (SIGINT). Letzter Stand in $LAST_OUTPUT_FILE"; exit 130' INT

mkdir -p "$RALPH_DIR"
LOG_FILE="$RALPH_DIR/ralph.log"
LAST_OUTPUT_FILE="$RALPH_DIR/last-output.txt"
ITERATION_FILE="$RALPH_DIR/iteration.txt"
mkdir -p "$RALPH_DIR"

# Git Worktree Isolation
WORKTREE_PATH=""
WORKTREE_BRANCH=""
if [[ $WORKTREE -eq 1 ]]; then
  if ! git rev-parse --is-inside-work-tree &>/dev/null; then
    echo "Fehler: --worktree benötigt ein Git-Repository."
    exit 1
  fi
  RUN_TS="$(date '+%Y%m%d-%H%M%S')"
  WORKTREE_BRANCH="ralph/run-${RUN_TS}"
  WORKTREE_PATH="$(git rev-parse --show-toplevel)/.ralph/worktrees/${RUN_TS}"
  mkdir -p "$(dirname "$WORKTREE_PATH")"
  git worktree add -b "$WORKTREE_BRANCH" "$WORKTREE_PATH" HEAD
  echo "🌿 Worktree erstellt: $WORKTREE_PATH (Branch: $WORKTREE_BRANCH)"
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
  # Change to worktree directory for the agent run
  cd "$WORKTREE_PATH"
fi

i=1
if [[ $RESUME -eq 1 && -f "$ITERATION_FILE" ]]; then
  saved=$(< "$ITERATION_FILE")
  if [[ "$saved" =~ ^[0-9]+$ && $saved -ge 1 && $saved -le $ITERATIONS ]]; then
    i=$saved
    echo "▶️  Resume: starte bei Iteration $i"
  fi
fi

printf '%s\n' "--- Ralph Konfiguration ---"
printf '  %-18s %s\n' "Iterationen:"  "$ITERATIONS"
printf '  %-18s %s\n' "Delay:"        "${DELAY}s"
printf '  %-18s %s\n' "Stop-Regex:"   "$STOP_REGEX"
printf '  %-18s %s\n' "Resume:"       "$( [[ $RESUME -eq 1 ]] && echo 'ja' || echo 'nein' )"
printf '  %-18s %s\n' "Worktree:"     "$( [[ $WORKTREE -eq 1 ]] && echo "$WORKTREE_PATH" || echo 'nein' )"
printf '  %-18s %s\n' "Log-Datei:"    "$LOG_FILE"
[[ -n "$EFFECTIVE_PROMPT_FILE" ]] && printf '  %-18s %s\n' "Prompt-Datei:" "$EFFECTIVE_PROMPT_FILE"
printf '  Kommando:          '
printf '%s ' "${CMD[@]}"; echo
echo ""

while [[ $i -le $ITERATIONS ]]; do
  echo "============================================================"
  echo "Iteration $i/$ITERATIONS"
  echo "============================================================"

  echo "$i" > "$ITERATION_FILE"

  set +e
  "${CMD[@]}" 2>&1 | tee "$LAST_OUTPUT_FILE"
  EXIT_CODE=${PIPESTATUS[0]}
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
      echo "📊 Änderungen seit letztem Commit (git diff --stat HEAD):"
      echo "$DIFF_STAT"
    fi
  fi

  if grep -Eiq "$STOP_REGEX" "$LAST_OUTPUT_FILE"; then
    echo "✅ Stopp-Bedingung erfüllt in Iteration $i"
    exit 0
  fi

  ((i++))
  sleep "$DELAY"
done

echo "⚠️ Max. Iterationen erreicht."
exit 2
