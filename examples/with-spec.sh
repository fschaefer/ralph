#!/usr/bin/env bash
# with-spec.sh – Load a named spec from .ralph/specs/
#
# Store reusable, task-specific prompts as named specs under .ralph/specs/.
# Use --spec <name> to load .ralph/specs/<name>.md and pass its path to the
# agent via the {SPEC_FILE} placeholder.
#
# Create a spec first:
#   mkdir -p .ralph/specs
#   cp my-feature-prompt.md .ralph/specs/auth.md
#
# Then run:

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$SCRIPT_DIR/../ralph.sh" \
  --spec auth \
  --max-iterations 10 \
  --delay 3 \
  -- claude -p @{SPEC_FILE}
