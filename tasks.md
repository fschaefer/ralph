# ralph – Task Checklist

## Setup
- [x] Create tasks.md (this file)
- [x] Create progress.txt

## Features (inspired by wiggum-cli)

### 1. Embed PROMPT_TEMPLATE.md in script
- [x] Add embedded default prompt template as heredoc inside ralph.sh
- [x] Fall back to embedded template when PROMPT_TEMPLATE.md is not found on disk
- [x] Keep external PROMPT_TEMPLATE.md working (takes priority over embedded)
- [x] Update README: document that no external template file is required

### 2. Action Inbox (pause-and-approve)
- [x] Add `--action-inbox` flag to ralph.sh
- [x] Detect `ACTION_REQUIRED: <message>` line in agent output
- [x] Pause loop, display message, prompt user for response (with optional timeout)
- [x] Write user response to `.ralph/inbox-response.txt` so the agent can read it next iteration
- [x] Update README with Action Inbox documentation
- [x] Add `examples/with-action-inbox.sh`

### 3. Run Summary
- [x] Track wall-clock start time of the run
- [x] Print elapsed total time at the end of the loop
- [x] Print per-iteration exit codes / status in summary
- [x] Print final outcome line (completed / max-iterations reached / interrupted)

### 4. Monitor Mode
- [x] Add `--monitor` subcommand / flag: tail `.ralph/ralph.log` in real-time
- [x] Show current iteration counter alongside tailed output
- [x] Update README with monitor documentation
- [x] Add `examples/monitor.sh`

### 5. Multiple Specs Support
- [x] Add `--spec <name>` flag: load `.ralph/specs/<name>.md` as prompt
- [x] Auto-substitute `{SPEC_FILE}` placeholder in agent command
- [x] Update README with specs documentation

### 6. Quiet Mode
- [x] Add `--quiet` / `-q` flag to suppress config header and iteration banners
- [x] Update README

### 7. Documentation & Examples
- [ ] Translate all to english
- [ ] Update README to cover all new features with examples
- [ ] Ensure all examples are consistent with new features
- [ ] Cleanup folder
- [ ] Use github.com/fschaefer/ralph
- [ ] add to prompt that .ralph directory should be excluded in .gitignore if git is used
