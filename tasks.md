# ralph.sh – Aufgabenliste (inspiriert von wiggum-cli)

## Features aus wiggum-cli, die portierbar sind

- [x] **SIGINT**: Trap Ctrl+C, log Unterbrechung, exit mit Code 130
- [x] **Delay**: `--delay <s>` Flag und `RALPH_DELAY` Umgebungsvariable (Standard 2s) für konfigurierbaren Sleep zwischen Iterationen
- [x] **Dry-run**: `--dry-run` Flag – Konfiguration ausgeben, ohne den Befehl auszuführen; exit 0
- [ ] **Resume**: `--resume` Flag – aktuelle Iteration in `.ralph/iteration.txt` speichern; beim nächsten Start mit `--resume` dort weitermachen
- [ ] **Named flags**: `--max-iterations <n>` und `--stop-regex <pattern>` als benannte Alternativen zu Positionalargument/Env-Var hinzufügen (Rückwärtskompatibilität bleibt erhalten)
- [ ] **Config-Display**: Formatierte Konfigurationsübersicht (Iterationen, Kommando, Delay, Regex) vor dem Schleifenstart ausgeben
