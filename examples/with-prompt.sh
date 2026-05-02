#!/usr/bin/env bash
# with-prompt.sh – Auto-generate a structured agent prompt via --goal and --stack
#
# ralph fills {{GOAL}} and {{STACK}} in PROMPT_TEMPLATE.md and saves the result
# to .ralph/PROMPT.md. The {PROMPT_FILE} placeholder in the agent command is
# replaced with the path to that file at runtime.
#
# Adjust --goal, --stack, --max-iterations, and the agent command to your needs.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$SCRIPT_DIR/../ralph.sh" \
  --goal "Build a REST API with CRUD endpoints for a todo list (title, done, created_at)" \
  --stack "Node.js 20, Express 4, SQLite (better-sqlite3), no TypeScript" \
  --max-iterations 10 \
  --delay 3 \
  -- claude -p @{PROMPT_FILE}
