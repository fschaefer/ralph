#!/usr/bin/env bash
# with-worktree.sh – Run the agent in an isolated Git worktree
#
# --worktree creates a fresh Git worktree under .ralph/worktrees/<timestamp>/
# on a new branch (ralph/run-<timestamp>). The agent works there without
# touching your main working tree. Useful for parallel runs or risky refactors.
#
# Requirements: the current directory must be inside a Git repository.

set -euo pipefail

ralph \
  --worktree \
  --goal "Refactor the authentication module to use JWT instead of sessions" \
  --stack "Python 3.11, FastAPI, python-jose, passlib" \
  --max-iterations 15 \
  --delay 5 \
  -- claude -p @{PROMPT_FILE}
