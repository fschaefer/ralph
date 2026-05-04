#!/usr/bin/env bash
# extend-spec.sh – Resume a completed project with new requirements
#
# Use --extend-spec <name> to reopen a project that has already output
# COMPLETE: true. ralph will append a new task to tasks.md referencing
# .ralph/specs/<name>.md so the agent picks up the new requirements on
# the next run.
#
# Create a spec file with the new requirements first:
#   mkdir -p .ralph/specs
#   cat > .ralph/specs/add-search.md <<'EOF'
#   Add a full-text search endpoint:
#   - GET /users/search?q=<query> – returns matching users
#   - Case-insensitive substring match on name and email
#   EOF
#
# Then run:

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$SCRIPT_DIR/../ralph.sh" \
  --extend-spec add-search \
  --max-iterations 10 \
  --delay 3 \
  -- claude -p @.ralph/PROMPT.md
