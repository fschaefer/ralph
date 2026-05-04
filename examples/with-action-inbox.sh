#!/usr/bin/env bash
# with-action-inbox.sh – Pause the loop and ask the user a question mid-run
#
# The agent can pause the loop and ask the user a question by outputting:
#   ACTION_REQUIRED: <your question here>
#
# ralph will print the message, wait for typed input, and save the response to
# .ralph/inbox-response.txt before continuing to the next iteration.
#
# The agent reads .ralph/inbox-response.txt on the next iteration to see the answer.
#
# Use --inbox-timeout to auto-continue after N seconds if the user does not respond.

set -euo pipefail

# Basic usage – wait indefinitely for user input
ralph \
  --action-inbox \
  --goal "Build a CLI tool" \
  --stack "Bash" \
  10 -- claude -p @{PROMPT_FILE}

# With a 90-second timeout – loop continues automatically if user doesn't respond
# ralph \
#   --action-inbox \
#   --inbox-timeout 90 \
#   --goal "Build a CLI tool" \
#   --stack "Bash" \
#   10 -- claude -p @{PROMPT_FILE}
