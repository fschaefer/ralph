#!/usr/bin/env bash
# with-prompt-file.sh – Use an existing prompt file as-is
#
# Point --prompt-file at a hand-written prompt and reference it through
# {PROMPT_FILE} in the agent command.

set -euo pipefail

ralph \
  8 \
  --prompt-file prompts/task.md \
  --delay 3 \
  -- claude -p @{PROMPT_FILE}
