#!/usr/bin/env bash
# with-prompt.sh – Auto-generate a structured agent prompt via --goal and --stack
#
# ralph fills {{GOAL}} and {{STACK}} in PROMPT_TEMPLATE.md and saves the result
# to .ralph/PROMPT.md. The {PROMPT_FILE} placeholder in the agent command is
# replaced with the path to that file at runtime.
#
# Adjust the iteration count, --goal, --stack, and the agent command to your needs.

set -euo pipefail

ralph \
  10 \
  --goal "Build a REST API with CRUD endpoints for a todo list (title, done, created_at)" \
  --stack "Node.js 20, Express 4, SQLite (better-sqlite3), no TypeScript" \
  --delay 3 \
  -- claude -p @{PROMPT_FILE}
