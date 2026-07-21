plugins {
    id("java")
    id("org.jetbrains.kotlin.jvm") version "2.0.21"
    id("org.jetbrains.intellij.platform") version "2.1.0"
}

group = "com.slopchop"
version = "0.1.2"

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    intellijPlatform {
        create("IC", "2024.2")
        // LSP4IJ provides the language-client plumbing this plugin registers a server with.
        plugin("com.redhat.devtools.lsp4ij", "0.7.0")
        instrumentationTools()
    }
}

intellijPlatform {
    pluginConfiguration {
        ideaVersion {
            sinceBuild = "242"
            untilBuild = provider { null }
        }
        changeNotes = """
            <ul>
              <li>Add the slop-chop plugin logo.</li>
              <li>Support every IDE build from 2024.2 onward, not just 2024.2.</li>
            </ul>
        """.trimIndent()
    }
}

kotlin {
    jvmToolchain(21)
}
