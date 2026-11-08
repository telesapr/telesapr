#!/bin/sh
# PostToolUse hook for Edit and Write: gofmt the Go file that was just
# written, as make fmt-check will require. Reads the tool call as JSON
# on stdin and does nothing for other files.
f=$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))' 2>/dev/null)
case "$f" in
  *.go) [ -f "$f" ] && gofmt -w "$f" ;;
esac
exit 0
