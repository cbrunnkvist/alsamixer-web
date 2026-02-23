# AGENT Guidelines for alsamixer-web Repository

This document outlines the build process, testing procedures, and coding style guidelines for contributing to the `alsamixer-web` project. Adhering to these guidelines ensures consistency, maintainability, and high quality across the codebase.

## 1. Build and Test Commands

All primary build and test operations are managed via the `Makefile`.

### Core Commands

*   **Build Project Locally**:
    ```bash
    make build
    ```
    This compiles the main `alsamixer-web` executable for the current operating system and architecture, placing it in the project root.

*   **Run Project Locally**:
    ```bash
    make run
    ```
    Executes the `alsamixer-web` server locally.

*   **Run All Tests Locally**:
    ```bash
    make test
    ```
    Executes all Go tests within the project.

*   **Clean Build Artifacts**:
    ```bash
    make clean
    ```
    Removes generated binaries (`alsamixer-web`) and the `dist/` directory.

### Cross-Compilation & Deployment

*   **Build for Linux AMD64**:
    ```bash
    make build-linux-amd64
    ```
    Cross-compiles the `alsamixer-web` binary for `linux/amd64` architecture, placing it in `dist/`.

*   **Build for Linux ARM64**:
    ```bash
    make build-linux-arm64
    ```
    Cross-compiles the `alsamixer-web` binary for `linux/arm64` architecture, placing it in `dist/`.

*   **Deploy to Remote Server**:
    ```bash
    make deploy DEPLOY_TARGET=user@host DEPLOY_PATH=/path/to/dest
    ```
    This target builds the `linux/amd64` binary and then uses `scp` to transfer it to the specified destination, also setting execute permissions. Replace `user@host` with your target server and `/path/to/dest` with the directory where you want to install the binary.

    Example usage:
    ```bash
    make deploy DEPLOY_TARGET=root@lemox.lan DEPLOY_PATH=/root/work/alsamixer-web
    ```

*   **Run Specific Go Test**:
    ```bash
    go test ./internal/server -run TestMyHandlerName
    ```
    Replace `./internal/server` with the path to the package and `TestMyHandlerName` with the exact test function name.

### E2E Tests with Playwright

The project includes E2E tests using Playwright to verify UI functionality. Tests require configuration via environment variables.

**Prerequisites:**
- Playwright is installed via `npm install` (dev dependency)
- Brave browser must be available at `/Applications/Brave Browser.app/Contents/MacOS/Brave Browser`

**Environment Variables:**
- `E2E_BASE_URL`: **Required** - URL of the running alsamixer-web server (e.g., `http://localhost:8888` or `http://lemox.lan:8888`)
- `E2E_SERVER_CMD_PREFIX`: Optional - Command prefix to run ALSA commands (e.g., `ssh lemox.lan` for remote server or `sudo` for local with privileges)

**Run All E2E Tests:**
```bash
E2E_BASE_URL=http://localhost:8888 node e2e.test.js
```

**Run Specific E2E Test File:**
```bash
# Basic UI tests
E2E_BASE_URL=http://localhost:8888 node e2e.test.js

# ALSA→UI synchronization tests
E2E_BASE_URL=http://lemox.lan:8888 E2E_SERVER_CMD_PREFIX="ssh lemox.lan" node e2e-alsa-to-ui.test.js

# Mute toggle tests
E2E_BASE_URL=http://lemox.lan:8888 E2E_SERVER_CMD_PREFIX="ssh lemox.lan" node e2e-mute.test.js
```

**Test Files:**
- `e2e.test.js` - Basic UI load and interaction tests
- `e2e-alsa-to-ui.test.js` - Tests external ALSA changes reflected in UI via SSE
- `e2e-mute.test.js` - Tests mute toggle UI↔ALSA synchronization

**Note:** Tests use Brave browser in non-headless mode for debugging. Modify the test files to use `headless: true` for CI/automated runs.

### Go Toolchain Commands

*   **Format Go Code**:
    ```bash
    go fmt ./...
    ```
    Ensures all Go files adhere to standard formatting.

*   **Run Static Analysis**:
    ```bash
    go vet ./...
    ```
    Performs basic static analysis to detect suspicious constructs.

## 2. Code Style Guidelines

### Go (`.go` files)

*   **Formatting**: Strictly adhere to `gofmt` output.
*   **Imports**:
    *   Group imports: standard library first, then external, then internal project packages.
    *   Use blank lines to separate import groups.
    *   Example:
        ```go
        import (
            "context"
            "fmt"
            "log"

            "github.com/external/library"

            "github.com/user/alsamixer-web/internal/alsa"
        )
        ```
*   **Naming Conventions**:
    *   **Packages**: `lowercase` (e.g., `server`, `alsa`).
    *   **Variables/Functions**: `camelCase` for unexported, `PascalCase` for exported.
    *   **Constants**: `PascalCase` for exported (e.g., `ThemeTerminal`), `camelCase` for unexported. `ALL_CAPS` for environment variables/flags.
    *   **Struct Fields**: `PascalCase` for exported, `camelCase` for unexported.
    *   **Interfaces**: Often end with `er` (e.g., `VolumeController`, `Hub`).
*   **Error Handling**:
    *   Always explicitly check errors (`if err != nil`).
    *   Wrap errors using `fmt.Errorf("descriptive message: %w", err)` for context.
    *   Return `nil, fmt.Errorf(...)` for functions that encounter errors.
    *   Use `log.Printf` for non-fatal errors and informational messages.
    *   Use `log.Fatalf` for unrecoverable errors during application startup.
    *   HTTP handlers should use `http.Error()` with appropriate `http.Status...` codes.
*   **Concurrency**:
    *   Protect shared state with `sync.Mutex` or other synchronization primitives.
    *   Use `go` goroutines for concurrent operations (e.g., SSE broadcasting, ALSA monitoring).
    *   Employ `sync.WaitGroup` for graceful shutdown of goroutines.
*   **Build Tags**: Use `//go:build <tag>` for platform-specific code (e.g., `//go:build linux` for ALSA integrations).
*   **Documentation**: Provide clear, concise comments for all exported functions, structs, interfaces, and complex logic blocks.

### HTML Templates (`.html` files)

*   **Structure**: Adhere to HTML5 standards with semantic tags.
*   **Accessibility (A11y)**:
    *   Prioritize ARIA attributes (`role`, `aria-label`, `aria-valuenow`, `aria-checked`, `aria-live`, etc.) for rich interactive elements.
    *   Include a skip link (`.skip-link`) for keyboard navigation.
*   **HTMX Integration**:
    *   Utilize `hx-post`, `hx-trigger`, `hx-swap`, `hx-vals` for dynamic interactions.
    *   `hx-on` for client-side JavaScript to handle post-request logic (e.g., updating `aria-checked`).
    *   Use `sse-connect` and `sse-swap` for Server-Sent Events.
*   **Go Template Syntax**:
    *   Use `{{define "name"}}...{{end}}` and `{{block "name" .}}...{{end}}` for template composition.
    *   Access data with `{{.FieldName}}`.
    *   Control flow with `{{if .Condition}}...{{end}}` and `{{range .Slice}}...{{end}}`.
    *   Include partials with `{{template "name" .}}`.

### CSS (`.css` files)

*   **Methodology**: Prefer BEM-like (`.block__element--modifier`) naming for clarity and modularity.
*   **CSS Custom Properties (Variables)**:
    *   Define global variables in `web/static/css/base.css`.
    *   Theme-specific variables (`--term-bg`, `--mobile-accent`) should override or extend base variables in their respective theme files (`web/static/themes/*.css`).
*   **Responsive Design**: Implement `@media` queries for different screen sizes (`max-width`, `min-width`) and user preferences (`prefers-reduced-motion`, `forced-colors`, `prefers-color-scheme`). Aim for a mobile-first approach.
*   **Accessibility**: Explicitly include styles for `:focus-visible` states, high contrast mode, and reduced motion.
*   **Clarity**: Start each CSS file with a clear, descriptive comment.

### JavaScript (`.js` files)

*   **Vanilla JS**: Prefer vanilla JavaScript over frameworks to keep the project lightweight.
*   **Encapsulation**: Use Immediately Invoked Function Expressions (IIFEs) for scope management.
*   **DOM Manipulation**: Direct DOM API usage (`querySelector`, `querySelectorAll`, `addEventListener`, `setAttribute`, `closest`).
*   **HTMX Interaction**: Listen for HTMX events (e.g., `htmx:afterSwap`) to re-initialize JavaScript logic on dynamically loaded content.
*   **Accessibility**: Directly update ARIA attributes in response to user interaction.
*   **Clarity**: Comment complex logic blocks and functions.

## 3. Tool-Specific Rules

No specific `.cursor/rules/` or `.github/copilot-instructions.md` files were found. Agents should infer best practices from the existing codebase and general industry standards.

### Playwright Browser Rule (Critical)

- **NEVER** run `browser_install` in this environment.
- Always use the existing Brave browser installation as the Chromium executable.
- Brave path (macOS):

  `/Applications/Brave Browser.app/Contents/MacOS/Brave Browser`

- Any Playwright session must explicitly use Brave instead of attempting to download or install Chrome/Chromium.

Failure to follow this rule will cause unnecessary timeouts and environment instability.

## 4. ALSA Reference

When working with ALSA mixer functionality, refer to `ALSA-NOTES.md` for:

- Core ALSA terminology (Cards, Controls, PCM devices, Channels)
- Control naming patterns and volume/switch derivation
- Channel configurations (Mono, Stereo Joined, Stereo Independent)
- Code patterns for GetMute/SetMute
- Common ALSA plugins (softvol, dmix, route, dsnoop)
- Debugging commands and lemox-specific setup

Key points:
- Switch values: ALSA 0 = muted, 1 = unmuted
- Derive switch name from volume: `strings.Replace(name, " Volume", " Switch", 1)`
- Always check channel configuration before assuming stereo/mono behavior
