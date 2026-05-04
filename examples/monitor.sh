#!/usr/bin/env bash
# examples/monitor.sh
#
# Run ralph in one terminal and watch the live log in another.
#
# Terminal 1 – start the run:
#   ralph --goal "Build a REST API" --stack "Node.js" 10 -- claude -p @{PROMPT_FILE}
#
# Terminal 2 – watch the live log:
#   ralph --monitor
#
# --monitor tails .ralph/ralph.log in real-time and shows which iteration is
# currently active. Press Ctrl+C to stop monitoring (the run continues).

ralph --monitor
