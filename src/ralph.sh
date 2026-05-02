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
  ./ralph.sh [--delay <s>] [--dry-run] [iterations] -- <agent-command...>

Optionen:
  --delay <s>   Pause zwischen Iterationen in Sekunden (Standard: 2, oder RALPH_DELAY)
  --dry-run     Konfiguration ausgeben, ohne den Befehl auszuführen; exit 0

Beispiel:
  ./ralph.sh 3 -- pi -p "Schreib einfach nur OK"
  ./ralph.sh --delay 5 3 -- pi -p "Schreib einfach nur OK"
  ./ralph.sh --dry-run 3 -- pi -p "Schreib einfach nur OK"

Hinweis:
  Standardmäßig stoppt der Loop bei einer Zeile wie:
    COMPLETE: true
  Regex anpassbar über STOP_REGEX.
EOF
}

ITERATIONS="5"
DELAY="${RALPH_DELAY:-2}"
DRY_RUN=0

# Parse named flags and optional positional iterations before '--'
while [[ $# -gt 0 && "${1:-}" != "--" ]]; do
  case "$1" in
    -h|--help)
      usage; exit 0
      ;;
    --delay)
      [[ -z "${2:-}" ]] && { echo "Fehler: --delay benötigt einen Wert."; exit 1; }
      DELAY="$2"; shift 2
      ;;
    --delay=*)
      DELAY="${1#*=}"; shift
      ;;
    --dry-run)
      DRY_RUN=1; shift
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
STOP_REGEX="${STOP_REGEX:-^COMPLETE:[[:space:]]*true$}"

if [[ $DRY_RUN -eq 1 ]]; then
  echo "🔍 Dry-run – Konfiguration (kein Befehl wird ausgeführt):"
  echo "- Iterationen: $ITERATIONS"
  echo "- Delay:       ${DELAY}s"
  echo "- Stop-Regex:  $STOP_REGEX"
  printf '%s ' "- Kommando:" "${CMD[@]}"; echo
  exit 0
fi

trap 'echo; echo "⚠️ Unterbrochen (SIGINT). Letzter Stand in $LAST_OUTPUT_FILE"; exit 130' INT

RALPH_DIR=".ralph"
LOG_FILE="$RALPH_DIR/ralph.log"
LAST_OUTPUT_FILE="$RALPH_DIR/last-output.txt"
mkdir -p "$RALPH_DIR"

echo "Starte Ralph Loop"
echo "- Iterationen: $ITERATIONS"
echo "- Delay:       ${DELAY}s"
printf '%s ' "- Kommando:" "${CMD[@]}"; echo

i=1
while [[ $i -le $ITERATIONS ]]; do
  echo "============================================================"
  echo "Iteration $i/$ITERATIONS"
  echo "============================================================"

  set +e
  "${CMD[@]}" 2>&1 | tee "$LAST_OUTPUT_FILE"
  EXIT_CODE=${PIPESTATUS[0]}
  set -e

  {
    echo "[$(date '+%F %T')] Iteration $i exit=$EXIT_CODE"
    cat "$LAST_OUTPUT_FILE"
    echo
  } >> "$LOG_FILE"

  if grep -Eiq "$STOP_REGEX" "$LAST_OUTPUT_FILE"; then
    echo "✅ Stopp-Bedingung erfüllt in Iteration $i"
    exit 0
  fi

  ((i++))
  sleep "$DELAY"
done

echo "⚠️ Max. Iterationen erreicht."
exit 2
