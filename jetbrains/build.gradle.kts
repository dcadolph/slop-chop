import org.jetbrains.intellij.platform.gradle.TestFrameworkType

plugins {
    id("java")
    id("org.jetbrains.kotlin.jvm") version "2.0.21"
    id("org.jetbrains.intellij.platform") version "2.1.0"
}

group = "com.slopchop"
version = "0.55.1"

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    // The platform test case runs on JUnit 4 under Gradle's test task. The platform's
    // assertions throw opentest4j failures, so that jar must be on the test classpath too.
    testImplementation("junit:junit:4.13.2")
    testImplementation("org.opentest4j:opentest4j:1.3.0")
    intellijPlatform {
        create("IC", "2024.2")
        // LSP4IJ provides the language-client plumbing this plugin registers a server with.
        plugin("com.redhat.devtools.lsp4ij", "0.7.0")
        instrumentationTools()
        testFramework(TestFrameworkType.Platform)
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
              <li>A setting under Tools, slop-chop, turns comment checking in source files off.</li>
              <li>The server runs in the project folder, so the project's .slop-chop.json
                  applies.</li>
            </ul>
        """.trimIndent()
    }
    publishing {
        // Read by the publishPlugin task, which the release workflow runs when the
        // JETBRAINS_MARKETPLACE_TOKEN secret is set. Unset locally, publishPlugin fails
        // with a clear missing-token error instead of uploading anything.
        token = providers.environmentVariable("JETBRAINS_MARKETPLACE_TOKEN")
    }
}

kotlin {
    jvmToolchain(21)
}
