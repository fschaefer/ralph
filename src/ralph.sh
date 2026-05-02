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
  ./ralph.sh [iterations] -- <agent-command...>

Beispiel:
  ./ralph.sh 3 -- pi -p "Schreib einfach nur OK"

Hinweis:
  Standardmäßig stoppt der Loop bei einer Zeile wie:
    COMPLETE: true
  Regex anpassbar über STOP_REGEX.
EOF
}

[[ "${1:-}" == "-h" || "${1:-}" == "--help" ]] && { usage; exit 0; }

ITERATIONS="5"
if [[ "${1:-}" != "--" ]]; then
  ITERATIONS="${1:-}"
  shift || true
fi

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

if [[ $# -lt 1 ]]; then
  echo "Fehler: Agent-Kommando fehlt."
  usage
  exit 1
fi

CMD=("$@")
STOP_REGEX="${STOP_REGEX:-^COMPLETE:[[:space:]]*true$}"

trap 'echo; echo "⚠️ Unterbrochen (SIGINT). Letzter Stand in $LAST_OUTPUT_FILE"; exit 130' INT

RALPH_DIR=".ralph"
LOG_FILE="$RALPH_DIR/ralph.log"
LAST_OUTPUT_FILE="$RALPH_DIR/last-output.txt"
mkdir -p "$RALPH_DIR"

echo "Starte Ralph Loop"
echo "- Iterationen: $ITERATIONS"
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
  sleep 2
done

echo "⚠️ Max. Iterationen erreicht."
exit 2
