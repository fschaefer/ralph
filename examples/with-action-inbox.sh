#!/usr/bin/env bash
# Action Inbox example
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

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

# Basic usage – wait indefinitely for user input
"$SCRIPT_DIR/ralph.sh" \
  --action-inbox \
  --goal "Build a CLI tool" \
  --stack "Bash" \
  10 -- claude -p @{PROMPT_FILE}

# With a 90-second timeout – loop continues automatically if user doesn't respond
# "$SCRIPT_DIR/ralph.sh" \
#   --action-inbox \
#   --inbox-timeout 90 \
#   --goal "Build a CLI tool" \
#   --stack "Bash" \
#   10 -- claude -p @{PROMPT_FILE}
