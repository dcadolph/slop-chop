# Editor plugin (LSP)

slop-chop can run as a Language Server, so any editor that speaks LSP can flag and chop slop
as you write. Tells show up as diagnostics, and the fix pass is offered two ways: a "Chop the
slop" code action, and document formatting, so format-on-save chops the file. It uses the same
profile, presets, and [voice](VOICE.md) as the command line.

## Run it

```
slop-chop lsp
```

It speaks the protocol on stdin and stdout. Point your editor's LSP client at that command for
Markdown, plain text, and commit messages. Add `--preset cleaver` for the aggressive swaps, or
`--voice path.json` for a specific voice; a `~/.slop-chop/voice.json` is picked up on its own.

## Neovim

With the built-in client:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = { "markdown", "text", "gitcommit" },
  callback = function()
    vim.lsp.start({
      name = "slop-chop",
      cmd = { "slop-chop", "lsp", "--preset", "cleaver" },
      root_dir = vim.fn.getcwd(),
    })
  end,
})
```

Diagnostics appear inline. Run `vim.lsp.buf.code_action` for "Chop the slop", or
`vim.lsp.buf.format` to chop the whole file.

## Helix

In `languages.toml`:

```toml
[language-server.slop-chop]
command = "slop-chop"
args = ["lsp", "--preset", "cleaver"]

[[language]]
name = "markdown"
language-servers = ["slop-chop"]
```

## VS Code

A ready-made extension lives in the repo under
[`vscode/`](https://github.com/dcadolph/slop-chop/tree/main/vscode). It runs the binary
directly: diagnostics as you type, a "Chop the slop" command, and a document formatter so
format-on-save chops the file. Its README covers the install. Rolling your own client
instead works too: point it at `slop-chop lsp` with a document selector for `markdown`,
`plaintext`, and `git-commit`.

## What it provides

- **Diagnostics** for every tell, with the rule name and the swap it would make.
- **Code action** "Chop the slop" to rewrite the whole document.
- **Formatting** that chops on demand or on save.
