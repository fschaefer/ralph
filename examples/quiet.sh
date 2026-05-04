#!/usr/bin/env bash
# quiet.sh – Run without config header and iteration banners
#
# --quiet / -q suppresses the "--- Ralph Configuration ---" header printed
# before the first iteration and the "===..." banners between iterations.
# All agent output and the final run summary are still printed.

set -euo pipefail

ralph \
  --quiet \
  --goal "Build a CLI tool that converts CSV to JSON" \
  --stack "Python 3, stdlib only" \
  --max-iterations 10 \
  -- claude -p @{PROMPT_FILE}
