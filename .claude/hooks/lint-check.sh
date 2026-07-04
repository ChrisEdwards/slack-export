#!/bin/bash
# Post tool use hook to run linter after file modifications
# Reports lint issues to Claude when they occur

set -o pipefail

# Read hook input from stdin
input=$(cat)

# Extract the file path from the tool input
file_path=$(echo "$input" | jq -r '.tool_input.file_path // empty')

# Only run on Go files
if [[ -z "$file_path" ]] || [[ ! "$file_path" =~ \.go$ ]]; then
    exit 0
fi

# Run make check and capture output
cd "$CLAUDE_PROJECT_DIR" || exit 0
output=$(make check 2>&1)
exit_code=$?

if [[ $exit_code -ne 0 ]]; then
    # Inform Claude about lint issues (file already written, just need fixes)
    echo "LINT ISSUES in $file_path - please fix:" >&2
    echo "$output" >&2
    exit 2
fi

exit 0
