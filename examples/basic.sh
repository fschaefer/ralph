#!/usr/bin/env bash
# basic.sh – Simplest ralph usage
#
# Runs an agent command up to 5 times.
# The loop stops early when the agent outputs a line matching "COMPLETE: true".
#
# Replace the inline prompt with your actual agent command.

set -euo pipefail

ralph \
  --max-iterations 5 \
  --delay 2 \
  -- claude -p "Fix the failing tests and print COMPLETE: true when done"
