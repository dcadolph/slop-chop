#!/bin/bash

# Chop the clipboard through slop-chop and copy the result back, with a notification. Wire it
# up as a SwiftBar or xbar plugin, an Automator Quick Action (Run Shell Script), or bind it to
# a hotkey with your launcher of choice.

set -euo pipefail

text="$(pbpaste)"
[ -z "${text//[[:space:]]/}" ] && exit 0

chopped="$(printf '%s' "$text" | slop-chop fix --preset cleaver)"
printf '%s' "$chopped" | pbcopy
osascript -e 'display notification "Clipboard chopped" with title "slop-chop"' >/dev/null 2>&1 || true
