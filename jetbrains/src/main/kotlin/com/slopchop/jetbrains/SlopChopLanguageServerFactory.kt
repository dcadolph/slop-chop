package com.slopchop.jetbrains

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.openapi.project.Project
import com.redhat.devtools.lsp4ij.LanguageServerFactory
import com.redhat.devtools.lsp4ij.server.OSProcessStreamConnectionProvider
import com.redhat.devtools.lsp4ij.server.StreamConnectionProvider

/**
 * SlopChopLanguageServerFactory registers the slop-chop LSP server with LSP4IJ. The server is
 * the local slop-chop binary run as `slop-chop lsp`, so the same rules engine that powers the
 * CLI, the web app, and the other editors backs JetBrains too.
 */
class SlopChopLanguageServerFactory : LanguageServerFactory {
    override fun createConnectionProvider(project: Project): StreamConnectionProvider {
        return SlopChopConnectionProvider()
    }
}

/**
 * SlopChopConnectionProvider launches `slop-chop lsp` and speaks LSP over its stdio. The binary
 * must be on the PATH; a missing binary surfaces as a server start error in the LSP4IJ console.
 */
class SlopChopConnectionProvider : OSProcessStreamConnectionProvider() {
    init {
        commandLine = GeneralCommandLine("slop-chop", "lsp")
    }
}
