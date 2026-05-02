# ralph.sh – Aufgabenliste (inspiriert von wiggum-cli)

## Bereits erledigt (vorherige Iterationen)

- [x] **SIGINT**: Trap Ctrl+C → exit 130, letzter Stand anzeigen
- [x] **Delay**: `--delay <s>` Flag + `RALPH_DELAY` Env-Var (Standard 2s)
- [x] **Dry-run**: `--dry-run` Flag – Konfiguration anzeigen, kein Befehl
- [x] **Resume**: `--resume` Flag – Iteration in `.ralph/iteration.txt` speichern
- [x] **Named flags**: `--max-iterations <n>` und `--stop-regex <pattern>`
- [x] **Config-Display**: Formatierte Konfigurationsübersicht vor Schleifenstart

## Neue Features

- [x] **Projektstruktur**: `ralph.sh` und `PROMPT_TEMPLATE.md` aus `src/` ins Root verschieben; `.gitignore` für `.ralph/` anlegen; `src/` löschen
- [x] **PROMPT-Integration**: `--goal <text>` und `--stack <text>` Flags; Template befüllen → `.ralph/PROMPT.md`; `{PROMPT_FILE}` Platzhalter in CMD-Args ersetzen; `--prompt-file <path>` Override
- [x] **--worktree**: Git Worktree Isolation für parallele Runs; separates Verzeichnis je Run
- [x] **Run Summary**: Nach jedem Loop kompakte Zusammenfassung via `git diff --stat` ausgeben
- [x] **README.md**: Installation, Usage, alle Flags, Beispiele, PROMPT-Integration erklärt
- [ ] **examples/**: Beispielskripte (`basic.sh`, `with-prompt.sh`, `with-worktree.sh`)
- [x] **PROMPT_TEMPLATE.md**: Platzhalter `{{GOAL}}` und `{{STACK}}` klar kennzeichnen; minimaler User-Input dokumentieren
