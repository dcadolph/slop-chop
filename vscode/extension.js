/* slop-chop for VS Code. Runs the local slop-chop binary over Markdown, plain text, and
   commit messages: tells show as diagnostics as you type, and a command or the formatter
   chops the document in place. Zero dependencies and no build step; the binary picks up
   your voice and profile the same way it does on the command line. */
"use strict";

const vscode = require("vscode");
const { execFile } = require("child_process");

// LANGUAGES is the set of documents the extension watches.
const LANGUAGES = ["markdown", "plaintext", "git-commit"];

// run feeds text to one slop-chop subcommand and resolves with stdout. check exits 1 on
// findings, which is a normal answer here, not a failure.
function run(args, text) {
  const cfg = vscode.workspace.getConfiguration("slop-chop");
  const bin = cfg.get("path") || "slop-chop";
  const preset = cfg.get("preset");
  const full = [...args];
  if (preset) full.push("--preset", preset);
  return new Promise((resolve, reject) => {
    const child = execFile(bin, full, { maxBuffer: 16 * 1024 * 1024 }, (err, stdout) => {
      if (err && err.code !== 1) {
        reject(new Error(String(err.message || err)));
        return;
      }
      resolve(stdout);
    });
    child.stdin.on("error", () => {});
    child.stdin.end(text);
  });
}

// byteToIndex maps a byte offset in the UTF-8 encoding of text to a JavaScript string index,
// which is what positionAt expects. Findings carry byte offsets.
function byteToIndex(text, byteOff) {
  let bytes = 0;
  for (let i = 0; i < text.length; i++) {
    if (bytes >= byteOff) return i;
    const code = text.codePointAt(i);
    bytes += code < 0x80 ? 1 : code < 0x800 ? 2 : code < 0x10000 ? 3 : 4;
    if (code > 0xffff) i++;
  }
  return text.length;
}

// toDiagnostics maps check findings onto VS Code diagnostics.
function toDiagnostics(doc, findings) {
  const text = doc.getText();
  return findings.map((f) => {
    const start = doc.positionAt(byteToIndex(text, f.offset));
    const end = doc.positionAt(byteToIndex(text, f.offset) + f.match.length);
    const msg =
      f.replacement == null
        ? `"${f.match}": ${f.rule}`
        : f.replacement === ""
          ? `"${f.match}": drop`
          : `"${f.match}" -> "${f.replacement}"`;
    const d = new vscode.Diagnostic(
      new vscode.Range(start, end),
      msg,
      vscode.DiagnosticSeverity.Information,
    );
    d.source = "slop-chop";
    d.code = f.rule;
    return d;
  });
}

// activate wires the diagnostics, the chop command, and the formatter.
function activate(context) {
  const collection = vscode.languages.createDiagnosticCollection("slop-chop");
  context.subscriptions.push(collection);
  const timers = new Map();

  // refresh re-checks one document and repaints its diagnostics.
  async function refresh(doc) {
    if (!LANGUAGES.includes(doc.languageId)) return;
    try {
      const out = await run(["check", "--json"], doc.getText());
      const report = JSON.parse(out);
      collection.set(doc.uri, toDiagnostics(doc, report.findings || []));
    } catch (err) {
      // A missing binary should say so once per session, not on every keystroke.
      collection.delete(doc.uri);
      if (!activate.warned) {
        activate.warned = true;
        vscode.window.showWarningMessage("slop-chop: " + err.message);
      }
    }
  }

  // debounced schedules a refresh shortly after typing stops.
  function debounced(doc) {
    const key = doc.uri.toString();
    clearTimeout(timers.get(key));
    timers.set(key, setTimeout(() => refresh(doc), 350));
  }

  context.subscriptions.push(
    vscode.workspace.onDidOpenTextDocument(refresh),
    vscode.workspace.onDidChangeTextDocument((e) => debounced(e.document)),
    vscode.workspace.onDidCloseTextDocument((doc) => collection.delete(doc.uri)),
  );
  vscode.workspace.textDocuments.forEach(refresh);

  // chop rewrites the whole document with the fix pass.
  async function chop(doc) {
    const out = await run(["fix"], doc.getText());
    return out;
  }

  context.subscriptions.push(
    vscode.commands.registerCommand("slop-chop.chop", async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor) return;
      try {
        const out = await chop(editor.document);
        const full = new vscode.Range(
          editor.document.positionAt(0),
          editor.document.positionAt(editor.document.getText().length),
        );
        await editor.edit((b) => b.replace(full, out));
      } catch (err) {
        vscode.window.showErrorMessage("slop-chop: " + err.message);
      }
    }),
    vscode.languages.registerDocumentFormattingEditProvider(LANGUAGES, {
      async provideDocumentFormattingEdits(doc) {
        try {
          const out = await chop(doc);
          if (out === doc.getText()) return [];
          const full = new vscode.Range(
            doc.positionAt(0),
            doc.positionAt(doc.getText().length),
          );
          return [vscode.TextEdit.replace(full, out)];
        } catch {
          return [];
        }
      },
    }),
  );
}

// deactivate has nothing to release; the disposables cover it.
function deactivate() {}

module.exports = { activate, deactivate };
