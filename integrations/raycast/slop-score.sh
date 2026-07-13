#!/bin/bash

# @raycast.schemaVersion 1
# @raycast.title Slop score (clipboard)
# @raycast.mode compact
# @raycast.icon 📈
# @raycast.packageName slop-chop
# @raycast.description Rate how much the clipboard reads like AI wrote it, 0 to 100.

set -euo pipefail

text="$(pbpaste)"
if [ -z "${text//[[:space:]]/}" ]; then
  echo "Clipboard is empty"
  exit 0
fi

echo "slop $(printf '%s' "$text" | slop-chop score --preset cleaver) / 100"
