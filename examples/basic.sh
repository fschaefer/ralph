#!/usr/bin/env bash
# basic.sh – Simplest ralph usage
#
# Runs an agent command up to 5 times.
# The loop stops early when the agent outputs a line matching "COMPLETE: true".
#
# Replace "claude -p @.ralph/PROMPT.md" with your actual agent command.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$SCRIPT_DIR/../ralph.sh" \
  --max-iterations 5 \
  --delay 2 \
  -- claude -p @.ralph/PROMPT.md
