# Integrations

Small ways to run slop-chop where you already work. These shell out to the `slop-chop` binary,
so install it first:

```
brew install dcadolph/tap/slop-chop
```

## Raycast

Script commands that act on the clipboard. Point Raycast at this folder (Extensions, then
Script Commands, then Add Script Directory), or copy the scripts into your own directory.

- **Chop clipboard** (`raycast/chop-clipboard.sh`) chops the clipboard and copies the result
  back, showing the slop score before and after.
- **Slop score** (`raycast/slop-score.sh`) rates the clipboard from 0 to 100.

## macOS

`macos/chop-clipboard.sh` chops the clipboard and copies it back with a notification. Wire it
up however suits you:

- **Quick Action.** In Automator, make a Quick Action, add Run Shell Script, set it to receive
  no input, and paste the script. It then appears under Services and can take a keyboard
  shortcut in System Settings, Keyboard, Shortcuts.
- **Chop the selection in place.** For a Quick Action that rewrites selected text, set Run
  Shell Script to receive text with "Output replaces selected text", and use just:
  `slop-chop fix --preset cleaver`.
- **Menu bar.** Drop the script into SwiftBar or xbar for a menu-bar chop.

## pre-commit

Flag slop in your commits with [pre-commit](https://pre-commit.com). In a repo's
`.pre-commit-config.yaml`:

```yaml
- repo: https://github.com/dcadolph/slop-chop
  rev: v0.23.0
  hooks:
    - id: slop-chop
```

It flags tells in Markdown and text files and fails when any are found. Add
`args: [--preset, cleaver]` for the aggressive list. To rewrite instead of flag, set
`entry: slop-chop fix --write` in your config's hook override.

## GitHub Action

Already built in, at the repo root (`action.yml`). It runs `check` or `fix` on a pull request.
See the [repo README](https://github.com/dcadolph/slop-chop).
