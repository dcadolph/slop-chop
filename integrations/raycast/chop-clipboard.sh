#!/bin/bash

# @raycast.schemaVersion 1
# @raycast.title Chop clipboard
# @raycast.mode compact
# @raycast.icon ✂️
# @raycast.packageName slop-chop
# @raycast.description Chop AI slop from the clipboard and copy the result back.

set -euo pipefail

text="$(pbpaste)"
if [ -z "${text//[[:space:]]/}" ]; then
  echo "Clipboard is empty"
  exit 0
fi

chopped="$(printf '%s' "$text" | slop-chop fix --preset cleaver)"
printf '%s' "$chopped" | pbcopy

before="$(printf '%s' "$text" | slop-chop score --preset cleaver)"
after="$(printf '%s' "$chopped" | slop-chop score --preset cleaver)"
echo "Chopped ✂ slop $before → $after (copied)"
