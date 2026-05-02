# ralph – Task-Checkliste

## Projekt-Ziel
ralph.sh ist ein minimaler Bash-Loop-Runner für autonome KI-Coding-Agenten, inspiriert von wiggum-cli.
Alle erreichbaren Features aus wiggum-cli sollen (soweit für ein Bash-Skript sinnvoll) integriert werden.

---

## Erledigte Tasks (aus progress.txt)

- [x] Projektstruktur: ralph.sh + PROMPT_TEMPLATE.md im Root, .gitignore für .ralph/
- [x] PROMPT-Integration: --goal, --stack, --prompt-file Flags; PROMPT_TEMPLATE.md → .ralph/PROMPT.md; {PROMPT_FILE} Platzhalter
- [x] --worktree: Isolierter Git Worktree für jeden Run
- [x] Run Summary: git diff --stat HEAD nach jeder Iteration
- [x] README.md: Vollständige Dokumentation
- [x] examples/: basic.sh, with-prompt.sh, with-worktree.sh
- [x] SIGINT-Trap, --delay, --dry-run, --resume, --max-iterations, --stop-regex, Config-Display

---

## Offene Tasks

- [x] --version Flag: Versionsnummer ausgeben und exit 0 (analog zu wiggum --version/-v)
- [ ] --timeout <s>: Pro-Iteration-Timeout mit `timeout`-Befehl; Agent-Prozess nach <s> Sekunden abbrechen; Fehler loggen; Loop fortsetzen (wiggum hat per-loop timeouts)
- [ ] SIGTERM-Trap: Neben SIGINT auch SIGTERM abfangen und sauber beenden (Cleanup wie bei SIGINT)
- [ ] Worktree-Cleanup: Worktree nach erfolgreichem Run automatisch aufräumen; --keep-worktree Flag um altes Verhalten beizubehalten
- [ ] {MODEL} Platzhalter in CMD-Args (analog zu {PROMPT_FILE}): --model <model> Flag; ersetzt {MODEL} im Agent-Kommando (z.B. `claude --model {MODEL} -p @{PROMPT_FILE}`)
- [ ] README + examples aktualisieren nach Implementierung der neuen Features
