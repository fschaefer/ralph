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
- [ ] Add `--action-inbox` flag to ralph.sh
- [ ] Detect `ACTION_REQUIRED: <message>` line in agent output
- [ ] Pause loop, display message, prompt user for response (with optional timeout)
- [ ] Write user response to `.ralph/inbox-response.txt` so the agent can read it next iteration
- [ ] Update README with Action Inbox documentation
- [ ] Add `examples/with-action-inbox.sh`

### 3. Run Summary
- [ ] Track wall-clock start time of the run
- [ ] Print elapsed total time at the end of the loop
- [ ] Print per-iteration exit codes / status in summary
- [ ] Print final outcome line (completed / max-iterations reached / interrupted)

### 4. Monitor Mode
- [ ] Add `--monitor` subcommand / flag: tail `.ralph/ralph.log` in real-time
- [ ] Show current iteration counter alongside tailed output
- [ ] Update README with monitor documentation
- [ ] Add `examples/monitor.sh`

### 5. Multiple Specs Support
- [ ] Add `--spec <name>` flag: load `.ralph/specs/<name>.md` as prompt
- [ ] Auto-substitute `{SPEC_FILE}` placeholder in agent command
- [ ] Update README with specs documentation

### 6. Quiet Mode
- [ ] Add `--quiet` / `-q` flag to suppress config header and iteration banners
- [ ] Update README

### 7. Documentation & Examples
- [ ] Update README to cover all new features with examples
- [ ] Ensure all examples are consistent with new features
