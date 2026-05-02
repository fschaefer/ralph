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
  --timeout <s>            Pro-Iteration-Timeout in Sekunden; Agent wird nach <s>s abgebrochen (0 = deaktiviert)
  --stop-regex <pattern>   Regex zum Erkennen des Stopp-Signals (Standard: STOP_REGEX oder ^COMPLETE:...)
  --action-inbox           Pausiere den Loop wenn der Agent "ACTION_REQUIRED: <msg>" ausgibt;
                           warte auf Benutzer-Eingabe und schreibe sie nach .ralph/inbox-response.txt
  --inbox-timeout <s>      Timeout für die Benutzer-Eingabe in Sekunden (0 = unbegrenzt, Standard: 0)
  --dry-run                Konfiguration ausgeben, ohne den Befehl auszuführen; exit 0
  --resume                 Bei der zuletzt gespeicherten Iteration weitermachen (.ralph/iteration.txt)
  --worktree               Isolierten Git Worktree für diesen Run erstellen (Branch: ralph/run-<ts>)
  --goal <text>            Projektziel (befüllt {{GOAL}} im Prompt-Template → .ralph/PROMPT.md)
  --stack <text>           Technologie-Stack (befüllt {{STACK}} im Prompt-Template → .ralph/PROMPT.md)
  --prompt-file <path>     Fertigen Prompt direkt übergeben (überschreibt --goal/--stack)
  -v, --version            Versionsnummer ausgeben und beenden

Prompt-Integration:
  Mit --goal und --stack wird das Prompt-Template befüllt und als .ralph/PROMPT.md gespeichert.
  Das Template ist im Skript eingebettet; eine externe PROMPT_TEMPLATE.md im Projektverzeichnis
  überschreibt das eingebettete Template.
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
  ./ralph.sh --timeout 120 5 -- claude -p @{PROMPT_FILE}
  ./ralph.sh --action-inbox 5 -- claude -p @{PROMPT_FILE}
  ./ralph.sh --action-inbox --inbox-timeout 120 5 -- claude -p @{PROMPT_FILE}

Hinweis:
  Standardmäßig stoppt der Loop bei einer Zeile wie:
    COMPLETE: true
  Regex anpassbar über --stop-regex oder STOP_REGEX.
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
    --timeout)
      [[ -z "${2:-}" ]] && { echo "Fehler: --timeout benötigt einen Wert."; exit 1; }
      TIMEOUT="$2"; shift 2
      ;;
    --timeout=*)
      TIMEOUT="${1#*=}"; shift
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
    --action-inbox)
      ACTION_INBOX=1; shift
      ;;
    --inbox-timeout)
      [[ -z "${2:-}" ]] && { echo "Fehler: --inbox-timeout benötigt einen Wert."; exit 1; }
      INBOX_TIMEOUT="$2"; shift 2
      ;;
    --inbox-timeout=*)
      INBOX_TIMEOUT="${1#*=}"; shift
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

if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]]; then
  echo "Fehler: timeout muss eine nicht-negative ganze Zahl sein."
  exit 1
fi

if ! [[ "$INBOX_TIMEOUT" =~ ^[0-9]+$ ]]; then
  echo "Fehler: inbox-timeout muss eine nicht-negative ganze Zahl sein."
  exit 1
fi

if [[ $# -lt 1 ]]; then
  echo "Fehler: Agent-Kommando fehlt."
  usage
  exit 1
fi

CMD=("$@")
STOP_REGEX="${STOP_REGEX_ARG:-${STOP_REGEX:-^COMPLETE:[[:space:]]*true$}}"

# Embedded default prompt template (used when no PROMPT_TEMPLATE.md is present on disk)
read -r -d '' EMBEDDED_PROMPT_TEMPLATE <<'EMBEDDED_EOF' || true
SYSTEM DIRECTIVE: AUTONOMOUS RALPH LOOP AGENT
Du bist ein autonomer Software-Engineering-Agent, der durch eine externe Bash-Schleife gesteuert wird. Du hast kein Gedächtnis zwischen den Iterationen. Dein gesamtes Wissen über das Projekt befindet sich ausschließlich im Dateisystem.

1. PROJEKTZIEL & SPEZIFIKATION

- Du baust folgendes Projekt:
{{GOAL}}

- Technologie-Stack & Architektur-Regeln:
{{STACK}}

2. STRIKTER WORKFLOW (IMMER BEFOLGEN!)
Befolge diese Schritte exakt in der angegebenen Reihenfolge. Überspringe keinen Schritt.

SCHRITT 1: Orientierung (State Recovery)

Lese tasks.md (die Todo-Liste) und progress.txt (das Log deiner Vorgänger).

Falls diese Dateien nicht existieren: Dies ist Iteration 1. Erstelle die grundlegende Projektstruktur. Erstelle eine tasks.md mit einer sehr granularen Checkliste basierend auf dem Projektziel. Erstelle eine leere progress.txt.

SCHRITT 2: Task-Auswahl

Identifiziere in der tasks.md den nächsten, einzelnen, logisch isolierten Task, der noch nicht erledigt ist.

Mache niemals mehrere komplexe Dinge gleichzeitig.

SCHRITT 3: Implementierung

Implementiere den ausgewählten Task. Schreibe oder refactore den entsprechenden Code.

SCHRITT 4: Backpressure & Verifikation (EXTREM WICHTIG)
Du darfst niemals davon ausgehen, dass dein Code funktioniert. Du musst zwingend externe Validierung nutzen.

Analysiere selbstständig die Projektstruktur (lese z. B. Dateien wie package.json, Makefile, Cargo.toml oder erkunde die Ordnerstruktur), um herauszufinden, welche Linter-, Type-Check- und Test-Befehle in diesem spezifischen Projekt verwendet werden.

Führe die so identifizierten Prüf-Befehle (z. B. npm test, tsc --noEmit, pytest) über dein Terminal-Tool aus.
Schlägt ein Test oder Linter fehl? Analysiere den Fehler und korrigiere den Code. Wenn du in einer Sackgasse steckst, dokumentiere es in Schritt 5 und beende dich für die nächste Iteration.

SCHRITT 5: Gedächtnis aktualisieren (Memory Injection)

Hänge an progress.txt einen kurzen Eintrag an: Welcher Task wurde bearbeitet? Welche Dateien wurden geändert? Gab es ungelöste Fehler? (Fasse dich kurz!).

Markiere den Task in der tasks.md nur dann als erledigt (z.B. [x]), wenn der Code geschrieben und durch die Befehle aus Schritt 4 erfolgreich und fehlerfrei verifiziert wurde.

SCHRITT 6: Git Commit

Führe über das Terminal aus: git add . gefolgt von git commit -m "ralph: task update"

SCHRITT 7: Terminierung

Szenario A (Es gibt noch offene Tasks oder Fehler): Beende deine Ausgabe mit einer kurzen Zusammenfassung. Der externe Loop wird dich für den nächsten Task neu starten.

Szenario B (ALLE Tasks sind erledigt UND verifiziert): Nur wenn absolut alle Anforderungen aus der tasks.md abgearbeitet sind und alle externen Checks fehlerfrei durchlaufen, gibst du exakt und als alleinstehende Zeile folgenden String aus:

COMPLETE: true
EMBEDDED_EOF

# Determine the effective prompt file path
RALPH_DIR=".ralph"
GENERATED_PROMPT_FILE="$RALPH_DIR/PROMPT.md"
EFFECTIVE_PROMPT_FILE=""

if [[ -n "$PROMPT_FILE_OVERRIDE" ]]; then
  EFFECTIVE_PROMPT_FILE="$PROMPT_FILE_OVERRIDE"
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
  echo "- Timeout:     $( [[ $TIMEOUT -gt 0 ]] && echo "${TIMEOUT}s" || echo 'deaktiviert' )"
  echo "- Stop-Regex:  $STOP_REGEX"
  echo "- Resume:      $( [[ $RESUME -eq 1 ]] && echo 'ja' || echo 'nein' )"
  echo "- Worktree:    $( [[ $WORKTREE -eq 1 ]] && echo 'ja' || echo 'nein' )"
  echo "- Action-Inbox: $( [[ $ACTION_INBOX -eq 1 ]] && echo "ja (timeout: $( [[ $INBOX_TIMEOUT -gt 0 ]] && echo "${INBOX_TIMEOUT}s" || echo 'unbegrenzt' ))" || echo 'nein' )"
  if [[ -n "$EFFECTIVE_PROMPT_FILE" ]]; then
    if [[ -n "$PROMPT_FILE_OVERRIDE" ]]; then
      echo "- Prompt-Datei: $EFFECTIVE_PROMPT_FILE (--prompt-file)"
    elif [[ -f "PROMPT_TEMPLATE.md" ]]; then
      echo "- Prompt-Datei: $EFFECTIVE_PROMPT_FILE (aus PROMPT_TEMPLATE.md)"
    else
      echo "- Prompt-Datei: $EFFECTIVE_PROMPT_FILE (eingebettetes Template)"
    fi
  fi
  printf '%s ' "- Kommando:" "${CMD[@]}"; echo
  exit 0
fi

ITER_STATUSES=()
RUN_START_TS=0

print_summary() {
  local outcome="${1:-unbekannt}"
  local elapsed=$(( $(date +%s) - RUN_START_TS ))
  local mins=$(( elapsed / 60 ))
  local secs=$(( elapsed % 60 ))
  echo ""
  echo "============================================================"
  echo "📋 Run-Zusammenfassung"
  echo "============================================================"
  printf '  %-20s %dm %02ds\n' "Gesamtdauer:" "$mins" "$secs"
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
  printf '  %-20s %s\n' "Ergebnis:" "$outcome"
  echo "============================================================"
}

trap 'echo; print_summary "⚠️  Unterbrochen (SIGINT)"; echo "Letzter Stand in $LAST_OUTPUT_FILE"; exit 130' INT

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
  INBOX_RESPONSE_FILE="$WT_RALPH_DIR/inbox-response.txt"
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
printf '  %-18s %s\n' "Timeout:"      "$( [[ $TIMEOUT -gt 0 ]] && echo "${TIMEOUT}s" || echo 'deaktiviert' )"
printf '  %-18s %s\n' "Stop-Regex:"   "$STOP_REGEX"
printf '  %-18s %s\n' "Resume:"       "$( [[ $RESUME -eq 1 ]] && echo 'ja' || echo 'nein' )"
printf '  %-18s %s\n' "Worktree:"     "$( [[ $WORKTREE -eq 1 ]] && echo "$WORKTREE_PATH" || echo 'nein' )"
printf '  %-18s %s\n' "Action-Inbox:" "$( [[ $ACTION_INBOX -eq 1 ]] && echo "ja (timeout: $( [[ $INBOX_TIMEOUT -gt 0 ]] && echo "${INBOX_TIMEOUT}s" || echo 'unbegrenzt' ))" || echo 'nein' )"
printf '  %-18s %s\n' "Log-Datei:"    "$LOG_FILE"
if [[ -n "$EFFECTIVE_PROMPT_FILE" ]]; then
  if [[ -n "$PROMPT_FILE_OVERRIDE" ]]; then
    printf '  %-18s %s\n' "Prompt-Datei:" "$EFFECTIVE_PROMPT_FILE (--prompt-file)"
  elif [[ -f "PROMPT_TEMPLATE.md" ]]; then
    printf '  %-18s %s\n' "Prompt-Datei:" "$EFFECTIVE_PROMPT_FILE (aus PROMPT_TEMPLATE.md)"
  else
    printf '  %-18s %s\n' "Prompt-Datei:" "$EFFECTIVE_PROMPT_FILE (eingebettetes Template)"
  fi
fi
printf '  Kommando:          '
printf '%s ' "${CMD[@]}"; echo
echo ""

RUN_START_TS=$(date +%s)

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

  STOP_MATCHED=0
  grep -Eiq "$STOP_REGEX" "$LAST_OUTPUT_FILE" && STOP_MATCHED=1 || true

  if [[ $STOP_MATCHED -eq 1 ]]; then
    _ITER_NOTE="✓ Stopp"
  elif [[ $EXIT_CODE -ne 0 ]]; then
    _ITER_NOTE="✗ Fehler"
  else
    _ITER_NOTE="→ weiter"
  fi
  ITER_STATUSES+=("$i:$EXIT_CODE:$_ITER_NOTE")

  if [[ $STOP_MATCHED -eq 1 ]]; then
    echo "✅ Stopp-Bedingung erfüllt in Iteration $i"
    print_summary "✅ Stopp-Bedingung erfüllt (Iteration $i)"
    exit 0
  fi

  if [[ $ACTION_INBOX -eq 1 ]]; then
    ACTION_LINE="$(grep -Eo 'ACTION_REQUIRED:[[:space:]]*.*' "$LAST_OUTPUT_FILE" | head -1 || true)"
    if [[ -n "$ACTION_LINE" ]]; then
      ACTION_MSG="${ACTION_LINE#ACTION_REQUIRED:}"
      ACTION_MSG="${ACTION_MSG#"${ACTION_MSG%%[![:space:]]*}"}"
      echo ""
      echo "📬 Action Inbox – Agent wartet auf Eingabe:"
      echo "   $ACTION_MSG"
      echo ""
      if [[ $INBOX_TIMEOUT -gt 0 ]]; then
        read -rp "Deine Antwort (${INBOX_TIMEOUT}s Timeout): " -t "$INBOX_TIMEOUT" USER_RESPONSE || {
          echo ""
          echo "⏱️ Timeout – keine Eingabe erhalten. Loop wird ohne Antwort fortgesetzt."
          USER_RESPONSE=""
        }
      else
        read -rp "Deine Antwort: " USER_RESPONSE
      fi
      echo "$USER_RESPONSE" > "$INBOX_RESPONSE_FILE"
      echo "✅ Antwort gespeichert in $INBOX_RESPONSE_FILE"
    fi
  fi

  ((i++))
  sleep "$DELAY"
done

echo "⚠️ Max. Iterationen erreicht."
print_summary "⚠️  Max. Iterationen ($ITERATIONS) erreicht"
exit 2
