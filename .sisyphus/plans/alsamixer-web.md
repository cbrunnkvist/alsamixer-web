# ALSA Mixer Web Interface - Work Plan

## TL;DR

> Build a lightweight, responsive web interface for ALSA mixer control using **Go backend** + **HTMX frontend** + **Server-Sent Events** for real-time bidirectional updates.
>
> **Deliverables**:
> - Go binary (`alsamixer-web`) with embedded static files
> - 5 themes: Terminal (ASCII), Modern Clean, Muji, Mobile-First, Creative
> - SSE-based real-time sync (<100ms latency)
> - asound.conf change detection
> - Self-healing network recovery
>
> **Effort**: Large (multi-component system)
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: Wave 1 (Core) → Wave 2 (UI) → Wave 3 (Themes) → Wave 4 (Polish)

---

## Context

### Project Goals
Create a web-based replacement for the terminal `alsamixer` program that:
- Provides real-time bidirectional volume control synchronization
- Works without HTTPS (plain HTTP)
- Uses lightweight technologies (no React/heavy frameworks)
- Supports 5 distinct visual themes
- Handles mobile and desktop layouts
- Self-heals from network interruptions

### Technical Decisions (Confirmed)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Backend** | Go | Performance, portability, single binary |
| **Frontend** | HTMX + SSE | Lightweight, no JS frameworks, works over HTTP |
| **Real-time** | Server-Sent Events | 96.44% browser support, HTTP-compatible, auto-reconnect |
| **ALSA Library** | `github.com/gen2brain/alsa` | Pure Go (no CGO), supports mixer controls |
| **Styling** | CSS Custom Properties | Runtime theme switching without rebuild |
| **Deployment** | Binary + Docker + Systemd | Flexible installation options |
| **Port** | 8080 (configurable) | 12-factor: default → env → CLI arg |

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│  Browser (Desktop/Mobile)                                   │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  HTML5 + HTMX 2.x + SSE Extension                     │ │
│  │  ┌─────────────────────────────────────────────────┐  │ │
│  │  │  <div hx-ext="sse" sse-connect="/events">       │  │ │
│  │  │    <!-- Controls updated via SSE -->            │  │ │
│  │  │  </div>                                         │  │ │
│  │  └─────────────────────────────────────────────────┘  │ │
│  └───────────────────────────────────────────────────────┘ │
└───────────────────────┬─────────────────────────────────────┘
                        │ HTTP/SSE (auto-reconnect on failure)
┌───────────────────────┴─────────────────────────────────────┐
│  Go Backend (single binary)                                 │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  HTTP Server (net/http or echo)                       │ │
│  │  ├── GET /           → index.html (theme selector)   │ │
│  │  ├── GET /static/*   → CSS, HTMX lib                  │ │
│  │  ├── GET /events     → SSE stream (mixer updates)    │ │
│  │  ├── POST /control/* → Volume/mute/toggle changes    │ │
│  │  └── GET /themes/*   → Theme CSS files               │ │
│  └───────────────────────────────────────────────────────┘ │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  ALSA Mixer Manager (gen2brain/alsa)                  │ │
│  │  - Enumerate cards & controls                        │ │
│  │  - Read/write volume levels                          │ │
│  │  - Monitor for changes (polling loop)                │ │
│  └───────────────────────────────────────────────────────┘ │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  File Watcher (asound.conf timestamp check)          │ │
│  │  - Check every 15 seconds                            │ │
│  │  - Notify UI when changed                            │ │
│  └───────────────────────────────────────────────────────┘ │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  SSE Broadcast Hub                                    │ │
│  │  - Manage client connections                         │ │
│  │  - Broadcast state changes                           │ │
│  │  - Handle reconnections gracefully                   │ │
│  └───────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

---

## Work Objectives

### Core Objective
Build a self-contained Go binary that serves a responsive web UI for controlling ALSA mixer volumes, with real-time synchronization between multiple clients and external volume changes.

### Concrete Deliverables
1. **Backend** (`cmd/alsamixer-web/`)
   - Go HTTP server with SSE support
   - ALSA mixer integration (gen2brain/alsa)
   - File watcher for asound.conf changes
   - State management and broadcasting

2. **Frontend** (`web/`)
   - HTMX-based HTML templates
   - 5 theme CSS files
   - Theme selector UI
   - Responsive layout (mobile-first)

3. **Static Assets** (embedded)
   - HTMX 2.x library
   - SSE extension
   - Theme stylesheets

4. **Deployment** (`deploy/`)
   - Dockerfile
   - Systemd service file
   - Build scripts

### Definition of Done
- [ ] All mixer controls from `alsamixer` are controllable via web UI
- [ ] Volume changes from other apps reflect in web UI within <100ms
- [ ] Web UI changes reflect in ALSA immediately
- [ ] 5 themes render correctly on desktop and mobile
- [ ] Network interruption recovery works (auto-reconnect)
- [ ] asound.conf change detection shows notification
- [ ] Binary runs standalone without dependencies

### Must Have
- Real-time bidirectional sync via SSE
- All themes functional and selectable
- Mobile-responsive layout
- asound.conf change detection
- Self-healing network behavior
- Single binary deployment
- Port configuration (default/env/CLI)
- **ARIA Accessibility: WCAG 2.1 AA compliance**
  - Semantic HTML5 structure
  - ARIA labels and roles for all controls
  - Keyboard navigation (Tab, Arrow keys, Enter, Space)
  - Focus indicators visible on all themes
  - Screen reader compatibility
  - High contrast support
  - ARIA live regions for dynamic updates

### Must NOT Have (Guardrails)
- NO authentication (reverse proxy handles this)
- NO HTTPS requirement (works on HTTP)
- NO React/Vue/Angular (HTMX only)
- NO WebSockets (SSE only)
- NO client-side state management (server-rendered)
- NO polling from client (SSE push only)

---

## Verification Strategy

### Test Infrastructure
- **Framework**: Go test + httptest for backend
- **Browser Testing**: Playwright for E2E
- **Manual QA**: Real ALSA hardware testing

### QA Policy
Every task includes agent-executed QA scenarios:

| Deliverable Type | Verification Tool | Method |
|------------------|-------------------|--------|
| Go backend | Go test + curl | Unit tests, API endpoint testing |
| SSE functionality | curl + browser | Stream validation, reconnection |
| UI rendering | Playwright | Cross-browser screenshots |
| Theme switching | Playwright | Visual regression |
| Mobile layout | Playwright (emulation) | Responsive breakpoint testing |

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation - Core Go Backend):
├── Task 1: Project structure and Go module setup
├── Task 2: ALSA mixer abstraction layer (gen2brain/alsa)
├── Task 3: SSE broadcast hub implementation
├── Task 4: File watcher for asound.conf
├── Task 5: HTTP server scaffolding + static file serving
└── Task 6: Configuration (port, env vars, CLI flags)

Wave 2 (UI Core - HTMX Templates & Basic Theme):
├── Task 7: HTMX integration and SSE connection
├── Task 8: Basic HTML templates (control rendering)
├── Task 9: Volume slider component (Terminal/ASCII theme)
├── Task 10: Mute/capture toggle components
├── Task 11: Card/control enumeration display
└── Task 12: Theme selector UI

Wave 3 (Themes - 5 Complete Theme Implementations):
├── Task 13: Terminal/ASCII theme (complete)
├── Task 14: Modern Clean theme
├── Task 15: Muji design theme
├── Task 16: Mobile-First theme (single control view)
├── Task 17: Creative theme
└── Task 18: Theme CSS variable system

Wave 4 (Integration & Polish):
├── Task 19: End-to-end integration (backend + frontend)
├── Task 20: Network interruption recovery logic
├── Task 21: asound.conf change notification UI
├── Task 22: Mobile responsiveness polish
├── Task 23: Build system (embed static files)
└── Task 24: Deployment artifacts (Docker, systemd)

Wave FINAL (Verification & Release):
├── Task F1: Plan compliance audit
├── Task F2: Code quality review
├── Task F3: Real hardware testing
└── Task F4: Documentation review

Critical Path: Wave 1 → Wave 2 → Wave 3 → Wave 4 → F1-F4
Parallel Speedup: 60% faster than sequential
Max Concurrent: 6 (Wave 1 & 3)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|------------|--------|------|
| 1-6 | - | 7-12 | 1 |
| 7-12 | 1-6 | 13-18 | 2 |
| 13-18 | 7-12 | 19-24 | 3 |
| 19-24 | 13-18 | F1-F4 | 4 |
| F1-F4 | 19-24 | - | FINAL |

---

## TODOs

- [x] 1. Project Structure and Go Module Setup

  **What to do**:
  - Initialize Go module: `go mod init github.com/user/alsamixer-web`
  - Create directory structure: `cmd/alsamixer-web/`, `internal/alsa/`, `internal/server/`, `internal/sse/`, `web/static/`, `web/templates/`
  - Set up build tags and embed directives
  - Create Makefile with build, test, run targets
  - Initialize git repository and add `references/alsa-utils` as submodule

  **Must NOT do**:
  - Don't write actual ALSA code yet
  - Don't create themes yet

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: None required
  - **Reason**: Standard project scaffolding task

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2-6)
  - **Blocks**: Tasks 2-6
  - **Blocked By**: None

  **References**:
  - Pattern: Standard Go project layout (https://github.com/golang-standards/project-layout)
  - Docs: Go embed package (https://pkg.go.dev/embed)

  **Acceptance Criteria**:
  - [ ] `go.mod` exists with module path
  - [ ] Directory structure created
  - [ ] Makefile with `build`, `test`, `run` targets
  - [ ] `go build ./...` succeeds
  - [ ] Git submodule `references/alsa-utils` added (read-only, for reference)

  **QA Scenarios**:
  ```
  Scenario: Build succeeds
    Tool: Bash
    Steps:
      1. Run `make build`
      2. Verify binary created at `./alsamixer-web`
    Expected: Binary exists and is executable
    Evidence: .sisyphus/evidence/task-1-build.png (ls -la output)
  ```

  **Commit**: YES
  - Message: `chore: initial project structure`
  - Files: All new files

---

- [x] 2. ALSA Mixer Abstraction Layer

  **What to do**:
  - Install dependency: `go get github.com/gen2brain/alsa`
  - Create `internal/alsa/mixer.go` with:
    - `Mixer` struct wrapping gen2brain/alsa
    - `ListCards()` - enumerate sound cards
    - `ListControls(card uint)` - enumerate mixer controls
    - `GetVolume(card uint, control string)` - get volume levels
    - `SetVolume(card uint, control string, values []int)` - set volume
    - `GetMute(card uint, control string)` - get mute state
    - `SetMute(card uint, control string, muted bool)` - set mute
    - `Close()` - cleanup
  - Handle errors gracefully (card not found, control not found)
  - Support both raw ALSA values and percentage (0-100)

  **Must NOT do**:
  - Don't implement event monitoring yet (Task 4)
  - Don't add SSE broadcasting here

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: None
  - **Reason**: Requires understanding ALSA API and gen2brain/alsa library

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3-6)
  - **Blocks**: Tasks 7, 19
  - **Blocked By**: Task 1

  **References**:
  - Code: `references/alsa-utils/alsamixer/mixer_controls.c` (alsamixer implementation)
  - API: https://pkg.go.dev/github.com/gen2brain/alsa
  - Docs: ALSA mixer API (https://www.alsa-project.org/alsa-doc/alsa-lib/group___mixer.html)

  **Acceptance Criteria**:
  - [ ] All methods implemented and tested
  - [ ] `go test ./internal/alsa/...` passes
  - [ ] Can enumerate cards and controls on real hardware
  - [ ] Volume get/set works correctly

  **QA Scenarios**:
  ```
  Scenario: Enumerate mixer controls
    Tool: Bash
    Preconditions: System has ALSA with at least one card
    Steps:
      1. Run `go test -v ./internal/alsa/...`
      2. Check test output shows detected cards and controls
    Expected: Tests pass and show real mixer controls
    Evidence: .sisyphus/evidence/task-2-mixer-test.txt
  ```

  **Commit**: YES
  - Message: `feat(alsa): mixer abstraction layer`
  - Files: `internal/alsa/*.go`

---

- [x] 3. SSE Broadcast Hub Implementation

  **What to do**:
  - Create `internal/sse/hub.go` with:
    - `Hub` struct managing client connections
    - `Register(client *Client)` - add new SSE client
    - `Unregister(client *Client)` - remove client
    - `Broadcast(event Event)` - send to all clients
    - `Run()` - main loop handling register/unregister/broadcast
  - Create `internal/sse/client.go` with:
    - `Client` struct (http.ResponseWriter, channel for events)
    - `WriteEvent(event Event)` - send event to client
    - `Close()` - cleanup
  - Event struct: `{Type string, Data interface{}, ID string}`
  - Handle client disconnections gracefully
  - Use mutex for thread-safe operations

  **Must NOT do**:
  - Don't integrate with HTTP handlers yet (Task 5)
  - Don't add ALSA-specific events yet

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: None
  - **Reason**: Concurrent programming, goroutines, channels

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-2, 4-6)
  - **Blocks**: Tasks 5, 19
  - **Blocked By**: Task 1

  **References**:
  - Pattern: Go chat server hub pattern
  - Docs: Go concurrency patterns (https://go.dev/doc/effective_go#concurrency)

  **Acceptance Criteria**:
  - [ ] Hub starts and stops cleanly
  - [ ] Multiple clients can connect
  - [ ] Broadcast reaches all clients
  - [ ] Disconnected clients are removed
  - [ ] Thread-safe (race detector: `go test -race`)

  **QA Scenarios**:
  ```
  Scenario: SSE hub works
    Tool: Bash
    Steps:
      1. Run hub tests: `go test -v -race ./internal/sse/...`
      2. Verify no race conditions
    Expected: Tests pass with -race flag
    Evidence: .sisyphus/evidence/task-3-sse-test.txt
  ```

  **Commit**: YES
  - Message: `feat(sse): broadcast hub implementation`
  - Files: `internal/sse/*.go`

---

- [x] 4. File Watcher for asound.conf Changes

  **What to do**:
  - Create `internal/config/watcher.go` with:
    - `Watcher` struct
    - `Start()` - begin watching
    - `Stop()` - stop watching
    - `OnChange(callback func())` - register change handler
  - Watch `/etc/asound.conf` and `~/.asoundrc`
  - Check modification time every 15 seconds (as specified)
  - Debounce changes (don't trigger multiple times for single save)
  - Send SSE event when change detected
  - UI should show notification banner

  **Must NOT do**:
  - Don't use inotify/fsnotify (keep it simple, polling is fine)
  - Don't restart the server automatically

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: None
  - **Reason**: Simple file polling implementation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-3, 5-6)
  - **Blocks**: Task 21
  - **Blocked By**: Task 1

  **References**:
  - Docs: Go os.Stat for file info
  - Pattern: Simple polling loop with ticker

  **Acceptance Criteria**:
  - [ ] Watcher detects file modifications
  - [ ] Callback triggered on change
  - [ ] No false positives
  - [ ] Unit tests with temp files

  **QA Scenarios**:
  ```
  Scenario: File change detection
    Tool: Bash
    Steps:
      1. Create temp file
      2. Start watcher
      3. Modify temp file
      4. Verify callback triggered within 15s
    Expected: Change detected
    Evidence: .sisyphus/evidence/task-4-watcher-test.txt
  ```

  **Commit**: YES
  - Message: `feat(config): asound.conf file watcher`
  - Files: `internal/config/watcher.go`

---

- [x] 5. HTTP Server Scaffolding and Static File Serving

  **What to do**:
  - Create `internal/server/server.go` with:
    - `Server` struct (addr string, hub *sse.Hub, mixer *alsa.Mixer)
    - `Start()` - start HTTP server
    - `Stop()` - graceful shutdown
  - Routes:
    - `GET /` - index handler (serve HTML)
    - `GET /events` - SSE endpoint
    - `POST /control/volume` - set volume
    - `POST /control/mute` - set mute
    - `GET /static/*` - static files
  - Embed static files using `//go:embed`
  - Graceful shutdown with context timeout

  **Must NOT do**:
  - Don't implement full handlers yet (just scaffolding)
  - Don't add templates yet

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: None
  - **Reason**: HTTP server setup, routing

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-4, 6)
  - **Blocks**: Tasks 7, 19
  - **Blocked By**: Tasks 1, 3

  **References**:
  - Docs: net/http package
  - Docs: Go embed directive
  - Pattern: Standard Go HTTP server setup

  **Acceptance Criteria**:
  - [ ] Server starts and listens on configured port
  - [ ] Static files served correctly
  - [ ] Graceful shutdown works
  - [ ] Routes return 200 (even if empty)

  **QA Scenarios**:
  ```
  Scenario: Server starts
    Tool: Bash
    Steps:
      1. Run `go run ./cmd/alsamixer-web &`
      2. Wait for "Server started" log
      3. Test `curl http://localhost:8080/`
      4. Kill server
    Expected: Server responds with 200
    Evidence: .sisyphus/evidence/task-5-server-start.txt
  ```

  **Commit**: YES
  - Message: `feat(server): HTTP server scaffolding`
  - Files: `internal/server/server.go`, `web/static/`

---

- [x] 6. Configuration (Port, Env Vars, CLI Flags)

  **What to do**:
  - Create `internal/config/config.go` with:
    - `Config` struct (Port int, LogLevel string, etc.)
    - `Load() (*Config, error)` - load configuration
  - Priority (highest to lowest):
    1. CLI flags (using flag package or cobra)
    2. Environment variables (e.g., `ALSAMIXER_WEB_PORT`)
    3. Default values (Port: 8080)
  - Support flags:
    - `--port` or `-p` (int)
    - `--bind` or `-b` (string, default "0.0.0.0")
    - `--alsacard` (uint, default 0)
  - Add usage/help text

  **Must NOT do**:
  - Don't use complex config file formats (YAML/JSON/TOML)
  - Keep it simple: flags + env only

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: None
  - **Reason**: Configuration loading logic

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-5)
  - **Blocks**: Task 5, 19
  - **Blocked By**: Task 1

  **References**:
  - Docs: Go flag package
  - Pattern: 12-factor app configuration

  **Acceptance Criteria**:
  - [ ] Default port 8080 works
  - [ ] Env var `ALSAMIXER_WEB_PORT` overrides default
  - [ ] CLI flag `--port` overrides env var
  - [ ] Help text shows all options

  **QA Scenarios**:
  ```
  Scenario: Configuration priority
    Tool: Bash
    Steps:
      1. Test default: `./alsamixer-web --help` shows port 8080
      2. Test env: `ALSAMIXER_WEB_PORT=9090 ./alsamixer-web` listens on 9090
      3. Test flag: `./alsamixer-web --port=31337` listens on 31337
    Expected: Priority respected
    Evidence: .sisyphus/evidence/task-6-config-test.txt
  ```

  **Commit**: YES
  - Message: `feat(config): CLI flags and environment variables`
  - Files: `internal/config/config.go`, `cmd/alsamixer-web/main.go`

---

- [ ] 7. HTMX Integration and SSE Connection

  **What to do**:
  - Download HTMX 2.x (latest stable) and SSE extension
  - Place in `web/static/js/htmx.min.js` and `web/static/js/htmx-sse.js`
  - Create base template `web/templates/base.html` with:
    - HTMX script tags
    - SSE extension script
    - CSS theme link
    - Meta tags for mobile viewport
  - Create `web/templates/index.html` extending base
  - Test SSE connection works (dummy events)

  **Must NOT do**:
  - Don't implement actual controls yet
  - Don't add themes yet (just placeholder)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: None
  - **Reason**: Frontend HTML/JS integration

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 8-12)
  - **Blocks**: Tasks 8-12
  - **Blocked By**: Wave 1

  **References**:
  - https://htmx.org/docs/
  - https://htmx.org/extensions/sse/
  - Pattern: HTMX + SSE examples from research

  **Acceptance Criteria**:
  - [ ] HTMX loaded (check browser console)
  - [ ] SSE connection established
  - [ ] Dummy events received and displayed

  **QA Scenarios**:
  ```
  Scenario: SSE connection works
    Tool: Playwright
    Steps:
      1. Navigate to http://localhost:8080
      2. Check browser console for "htmx:sseOpen" event
      3. Verify EventSource connection in Network tab
    Expected: Connection established
    Evidence: .sisyphus/evidence/task-7-sse-connection.png
  ```

  **Commit**: YES
  - Message: `feat(web): HTMX and SSE integration`
  - Files: `web/static/js/`, `web/templates/base.html`

---

- [ ] 8. Basic HTML Templates (Control Rendering)

  **What to do**:
  - Create `web/templates/controls.html` with:
    - Card container for each sound card
    - Control list within each card
    - Volume control template (slider input)
    - Mute toggle template (checkbox)
    - Capture toggle template
  - **ACCESSIBILITY: Implement semantic HTML and ARIA:**
    - Use `<section>` for cards with `aria-labelledby`
    - Use `<h2>` for card titles (proper heading hierarchy)
    - Volume controls: `role="slider"` with `aria-valuemin/max/now`
    - Toggle buttons: `role="switch"` with `aria-checked`
    - Add `aria-label` with descriptive names (e.g., "Master volume")
    - Add hidden live region: `<div aria-live="polite" class="sr-only">` for SSE announcements
    - Ensure all interactive elements are keyboard accessible (`tabindex` where needed)
    - Add skip link: `<a href="#main">Skip to main content</a>`
  - Server-side render using Go templates
  - Pass mixer state to template
  - Use HTMX `hx-post` for user interactions

  **Must NOT do**:
  - Don't style yet (Task 9+)
  - Don't add theme switching yet
  - Don't use generic `<div>` for buttons (use `<button>`)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: None
  - **Reason**: HTML templating, Go templates, accessibility

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7, 9-12)
  - **Blocks**: Tasks 9-12
  - **Blocked By**: Tasks 2, 5, 7

  **References**:
  - Docs: Go html/template package
  - Pattern: Server-side rendering with HTMX
  - ARIA: https://www.w3.org/WAI/ARIA/apg/patterns/slider/
  - ARIA: https://www.w3.org/WAI/ARIA/apg/patterns/switch/

  **Acceptance Criteria**:
  - [ ] All mixer controls rendered
  - [ ] Control names displayed
  - [ ] Current values shown
  - [ ] HTMX attributes on interactive elements
  - [ ] **Accessibility:**
    - [ ] Semantic HTML structure (`<section>`, `<h2>`, `<button>`)
    - [ ] ARIA roles on all controls (slider, switch)
    - [ ] ARIA labels describe each control
    - [ ] ARIA live region for announcements
    - [ ] Skip link present
    - [ ] Keyboard accessible (tabindex)

  **QA Scenarios**:
  ```
  Scenario: Controls rendered
    Tool: Playwright
    Steps:
      1. Navigate to http://localhost:8080
      2. Verify mixer cards displayed
      3. Verify control names and values visible
    Expected: All controls from alsamixer visible
    Evidence: .sisyphus/evidence/task-8-controls.png
  
  Scenario: Accessibility audit
    Tool: Playwright + axe-core
    Steps:
      1. Run axe accessibility scan
      2. Check for critical/serious violations
    Expected: 0 violations
    Evidence: .sisyphus/evidence/task-8-a11y-report.json
  
  Scenario: Keyboard navigation
    Tool: Playwright
    Steps:
      1. Press Tab to navigate through controls
      2. Verify focus moves logically
      3. Press Space to toggle mute
      4. Press Arrow keys to adjust volume
    Expected: Full keyboard control works
    Evidence: .sisyphus/evidence/task-8-keyboard.mp4
  ```

  **Commit**: YES
  - Message: `feat(web): basic control templates with ARIA accessibility`
  - Files: `web/templates/controls.html`, `internal/server/handlers.go`, `web/static/css/accessibility.css`

---

- [ ] 9. Volume Slider Component (Terminal/ASCII Theme)

  **What to do**:
  - Create `web/static/themes/terminal.css`
  - Style volume sliders as ASCII blocks (████░░░░░)
  - Use CSS custom properties for colors
  - Slider appearance:
    - Vertical or horizontal bars
    - Block characters or div-based bars
    - Current value displayed numerically
  - Interactive:
    - Click to set volume
    - Drag to adjust
    - HTMX triggers POST on change

  **Must NOT do**:
  - Don't use native HTML range input (custom styled)
  - Don't add animations yet

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: None
  - **Reason**: CSS styling, custom UI components

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-8, 10-12)
  - **Blocks**: Task 13
  - **Blocked By**: Task 8

  **References**:
  - Pattern: Terminal alsamixer UI (references/alsa-utils/alsamixer/)
  - CSS: Custom properties for theming

  **Acceptance Criteria**:
  - [ ] Sliders look like ASCII bars
  - [ ] Click/drag adjusts volume
  - [ ] Visual feedback on interaction
  - [ ] Works with SSE updates

  **QA Scenarios**:
  ```
  Scenario: Volume slider works
    Tool: Playwright
    Steps:
      1. Navigate to http://localhost:8080
      2. Click on volume bar at 75%
      3. Verify SSE event sent
      4. Verify visual update
    Expected: Volume changes and UI updates
    Evidence: .sisyphus/evidence/task-9-slider.mp4 (screen recording)
  ```

  **Commit**: YES
  - Message: `feat(theme): terminal/ASCII volume slider`
  - Files: `web/static/themes/terminal.css`

---

- [ ] 10. Mute/Capture Toggle Components

  **What to do**:
  - Create toggle button styles in terminal.css
  - Mute button: [M] shows when muted
  - Capture button: [C] shows when capturing
  - Visual states:
    - Active: highlighted
    - Inactive: dimmed
  - HTMX: POST on click to toggle
  - SSE: Update when changed externally

  **Must NOT do**:
  - Don't use native checkboxes (custom styled)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: None
  - **Reason**: Interactive component styling

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-9, 11-12)
  - **Blocks**: Task 13
  - **Blocked By**: Task 8

  **References**:
  - Pattern: alsamixer toggle display (references/alsa-utils/)

  **Acceptance Criteria**:
  - [ ] Mute toggle works
  - [ ] Capture toggle works
  - [ ] Visual state reflects actual state
  - [ ] External changes reflected via SSE

  **QA Scenarios**:
  ```
  Scenario: Toggle mute
    Tool: Playwright
    Steps:
      1. Click mute button
      2. Verify state change sent
      3. Use amixer to change mute externally
      4. Verify UI updates via SSE
    Expected: Bidirectional sync works
    Evidence: .sisyphus/evidence/task-10-toggle.mp4
  ```

  **Commit**: YES
  - Message: `feat(theme): mute and capture toggles`
  - Files: `web/static/themes/terminal.css` (updates)

---

- [ ] 11. Card/Control Enumeration Display

  **What to do**:
  - Display sound cards as sections/containers
  - Show card name and number
  - Group controls logically within each card
  - Handle cards with many controls (scrolling)
  - Show control type icons/indicators

  **Must NOT do**:
  - Don't add advanced grouping yet

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: None
  - **Reason**: Layout and organization

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-10, 12)
  - **Blocks**: None
  - **Blocked By**: Task 8

  **Acceptance Criteria**:
  - [ ] Cards displayed as sections
  - [ ] Card names shown
  - [ ] Controls grouped under cards
  - [ ] Layout responsive

  **QA Scenarios**:
  ```
  Scenario: Cards displayed
    Tool: Playwright
    Steps:
      1. Navigate to UI
      2. Verify each card has header
      3. Verify controls under correct card
    Expected: Proper hierarchy
    Evidence: .sisyphus/evidence/task-11-cards.png
  ```

  **Commit**: GROUP with Task 8
  - Message: `feat(web): card and control display`

---

- [ ] 12. Theme Selector UI

  **What to do**:
  - Add theme selector dropdown/button to header
  - List all available themes
  - On selection: reload page with new theme
  - Store selection in URL query (?theme=modern)
  - Default theme if none selected

  **Must NOT do**:
  - Don't use JavaScript for theme switching (reload is OK per requirements)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: None
  - **Reason**: Simple UI element

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7-11)
  - **Blocks**: Tasks 13-18
  - **Blocked By**: Task 7

  **Acceptance Criteria**:
  - [ ] Theme selector visible
  - [ ] All themes listed
  - [ ] Selection changes theme on reload
  - [ ] Selection persists in URL

  **QA Scenarios**:
  ```
  Scenario: Theme switching
    Tool: Playwright
    Steps:
      1. Open UI with default theme
      2. Select "modern" from dropdown
      3. Verify page reloads
      4. Verify URL has ?theme=modern
      5. Verify styling changed
    Expected: Theme switch works
    Evidence: .sisyphus/evidence/task-12-theme-switch.png
  ```

  **Commit**: YES
  - Message: `feat(web): theme selector`
  - Files: `web/templates/base.html` (selector), `internal/server/handlers.go`

---

- [ ] 13. Terminal/ASCII Theme (Complete)

  **What to do**:
  - Complete `web/static/themes/terminal.css`
  - Design goals:
    - Mimic classic alsamixer terminal look
    - Monospace font (Courier New or similar)
    - Block characters (█, ░) for volume bars
    - ASCII-style borders (┌─┐│└┘)
    - Black/dark background, green/white text
  - Components:
    - Card headers with box drawing
    - Volume bars using block characters
    - [M] and [C] toggles
    - Responsive: horizontal scroll on mobile
  - CSS variables for easy customization
  - **ACCESSIBILITY (Critical for Terminal Theme):**
    - High contrast: Ensure text meets WCAG 4.5:1 ratio
    - Focus indicators: Bright outline on dark background (lime green: #0f0)
    - Block characters must have ARIA labels (screen readers can't read "█")
    - Volume percentage always visible (not just block bars)
    - Support Windows High Contrast Mode: `forced-colors: active`
    - Keyboard focus must be highly visible (inverse colors or bright border)
    - Test with screen reader to verify block characters announced properly

  **Must NOT do**:
  - Don't use actual Unicode box drawing if browser support issues (use CSS borders)
  - Don't sacrifice contrast for aesthetic (must meet WCAG AA)

  **Recommended Agent Profile**:
  - **Category**: `artistry`
  - **Skills**: None
  - **Reason**: Creative theme design with accessibility focus

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 14-18)
  - **Blocks**: None
  - **Blocked By**: Tasks 9-10

  **References**:
  - Visual: Run `alsamixer` in terminal for reference
  - CSS: Box drawing characters, monospace fonts
  - ARIA: https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html
  - ARIA: https://www.w3.org/WAI/WCAG21/Understanding/non-text-contrast.html

  **Acceptance Criteria**:
  - [ ] Looks like terminal alsamixer
  - [ ] Works on desktop and mobile
  - [ ] All controls styled consistently
  - [ ] Block characters render correctly
  - [ ] **Accessibility:**
    - [ ] WCAG contrast ratio 4.5:1 minimum (use contrast checker)
    - [ ] Focus indicators highly visible (outline: 2px solid #0f0)
    - [ ] Screen reader announces volume values (not just blocks)
    - [ ] Windows High Contrast Mode supported
    - [ ] Keyboard navigation clearly visible

  **QA Scenarios**:
  ```
  Scenario: Terminal theme renders
    Tool: Playwright
    Steps:
      1. Load UI with ?theme=terminal
      2. Screenshot at desktop size
      3. Screenshot at mobile size
      4. Compare to actual alsamixer
    Expected: Visual similarity to terminal
    Evidence: .sisyphus/evidence/task-13-terminal-theme.png
  
  Scenario: Terminal theme contrast check
    Tool: Lighthouse or axe DevTools
    Steps:
      1. Run accessibility audit
      2. Check contrast ratios
    Expected: All text meets 4.5:1, UI elements 3:1
    Evidence: .sisyphus/evidence/task-13-contrast-report.json
  
  Scenario: Terminal theme focus visibility
    Tool: Playwright
    Steps:
      1. Load terminal theme
      2. Press Tab to focus first control
      3. Screenshot focused state
    Expected: Clear visual focus indicator
    Evidence: .sisyphus/evidence/task-13-focus.png
  ```

  **Commit**: YES
  - Message: `feat(theme): complete terminal/ASCII theme with accessibility`
  - Files: `web/static/themes/terminal.css`

---

- [ ] 14. Modern Clean Theme

  **What to do**:
  - Create `web/static/themes/modern.css`
  - Design goals:
    - Clean, minimalist aesthetic
    - Light gray/white background
    - Subtle shadows and rounded corners
    - Modern sans-serif font (system-ui, sans-serif)
    - Smooth transitions
  - Components:
    - Cards with subtle shadows
    - Modern range sliders (styled input[type=range])
    - Toggle switches (styled checkboxes)
    - Clean typography
  - Mobile: Stack cards vertically

  **Must NOT do**:
  - Don't over-design (keep it simple)

  **Recommended Agent Profile**:
  - **Category**: `artistry`
  - **Skills**: None
  - **Reason**: Modern UI design

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 13, 15-18)
  - **Blocks**: None
  - **Blocked By**: Task 8

  **References**:
  - Inspiration: Modern web apps, Material Design Lite

  **Acceptance Criteria**:
  - [ ] Clean, professional appearance
  - [ ] All controls styled consistently
  - [ ] Mobile-responsive
  - [ ] Accessible contrast ratios

  **QA Scenarios**:
  ```
  Scenario: Modern theme renders
    Tool: Playwright
    Steps:
      1. Load UI with ?theme=modern
      2. Verify clean appearance
      3. Test mobile viewport
    Expected: Modern, clean UI
    Evidence: .sisyphus/evidence/task-14-modern-theme.png
  ```

  **Commit**: YES
  - Message: `feat(theme): modern clean theme`
  - Files: `web/static/themes/modern.css`

---

- [ ] 15. Muji Design Theme

  **What to do**:
  - Create `web/static/themes/muji.css`
  - Design goals (Muji aesthetic):
    - Neutral, earthy color palette (beige, brown, off-white)
    - Minimal ornamentation
    - Natural textures feel
    - Focus on function over form
    - Generous whitespace
  - Components:
    - Subtle card separation (no heavy borders)
    - Natural color volume bars (wood-like or neutral)
    - Simple, clear typography
    - Unobtrusive controls
  - Philosophy: "No-brand quality goods"

  **Must NOT do**:
  - Don't use bright colors
  - Don't add decorative elements

  **Recommended Agent Profile**:
  - **Category**: `artistry`
  - **Skills**: None
  - **Reason**: Specific design aesthetic

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 13-14, 16-18)
  - **Blocks**: None
  - **Blocked By**: Task 8

  **References**:
  - Inspiration: Muji brand aesthetic
  - Colors: #F5F5DC (beige), #8B7355 (brown), #FFFEF0 (off-white)

  **Acceptance Criteria**:
  - [ ] Neutral color palette
  - [ ] Minimal, unbranded appearance
  - [ ] Relaxed, calm feel
  - [ ] Functional clarity

  **QA Scenarios**:
  ```
  Scenario: Muji theme renders
    Tool: Playwright
    Steps:
      1. Load UI with ?theme=muji
      2. Verify neutral colors
      3. Check whitespace usage
    Expected: Muji aesthetic achieved
    Evidence: .sisyphus/evidence/task-15-muji-theme.png
  ```

  **Commit**: YES
  - Message: `feat(theme): Muji design theme`
  - Files: `web/static/themes/muji.css`

---

- [ ] 16. Mobile-First Theme

  **What to do**:
  - Create `web/static/themes/mobile.css`
  - Design goals:
    - Optimized for small screens
    - Single control per view (swipe/pagination)
    - Large touch targets (min 44x44px)
    - Bottom navigation bar
    - Swipe gestures for navigation
  - Components:
    - Full-width cards
    - Large volume slider (vertical or large horizontal)
    - Big toggle buttons
    - Clear typography (readable at arm's length)
    - Swipe indicator (dots)
  - Multiple panels that can be swiped:
    - Show one control at a time
    - Dots indicate position
    - Swipe or click arrows to navigate

  **Must NOT do**:
  - Don't sacrifice desktop usability (responsive approach)

  **Recommended Agent Profile**:
  - **Category**: `artistry`
  - **Skills**: None
  - **Reason**: Mobile-specific UX design

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 13-15, 17-18)
  - **Blocks**: None
  - **Blocked By**: Task 8

  **References**:
  - Pattern: Mobile audio apps (iOS Music app, etc.)
  - UX: Touch targets, swipe gestures

  **Acceptance Criteria**:
  - [ ] Single control per view on mobile
  - [ ] Swipe/pagination works
  - [ ] Large touch targets
  - [ ] Readable on small screens
  - [ ] Desktop still usable (side-by-side or list)

  **QA Scenarios**:
  ```
  Scenario: Mobile theme works on phone
    Tool: Playwright (mobile emulation)
    Steps:
      1. Emulate iPhone 12 (390x844)
      2. Load ?theme=mobile
      3. Verify one control visible
      4. Swipe to next control
      5. Verify navigation dots
    Expected: Mobile-optimized UI
    Evidence: .sisyphus/evidence/task-16-mobile-theme.mp4
  ```

  **Commit**: YES
  - Message: `feat(theme): mobile-first theme`
  - Files: `web/static/themes/mobile.css`, `web/static/js/mobile.js` (swipe logic)

---

- [ ] 17. Creative Theme

  **What to do**:
  - Create `web/static/themes/creative.css`
  - Design goals (creative/fun):
    - Distinctive, memorable aesthetic
    - Could be:
      - Dark mode with neon accents
      - Retro 80s synthwave style
      - Brutalist design
      - Glassmorphism
      - Or user suggestion
    - Bold and different from other themes
    - Still usable and functional
  - Components:
    - Eye-catching volume visualization
    - Unique toggle designs
    - Distinctive color scheme
    - Memorable interactions

  **Must NOT do**:
  - Don't sacrifice usability for aesthetics
  - Don't make it confusing

  **Recommended Agent Profile**:
  - **Category**: `artistry`
  - **Skills**: None
  - **Reason**: Creative design exploration

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 13-16, 18)
  - **Blocks**: None
  - **Blocked By**: Task 8

  **References**:
  - Inspiration: Synthwave, brutalism, glassmorphism trends

  **Acceptance Criteria**:
  - [ ] Distinctive from other themes
  - [ ] Memorable visual style
  - [ ] Still functional and usable
  - [ ] Mobile-responsive

  **QA Scenarios**:
  ```
  Scenario: Creative theme renders
    Tool: Playwright
    Steps:
      1. Load ?theme=creative
      2. Verify distinctive style
      3. Test all controls
    Expected: Creative, unique UI
    Evidence: .sisyphus/evidence/task-17-creative-theme.png
  ```

  **Commit**: YES
  - Message: `feat(theme): creative theme`
  - Files: `web/static/themes/creative.css`

---

- [ ] 18. Theme CSS Variable System

  **What to do**:
  - Create `web/static/themes/base.css` with:
    - Common CSS custom properties (--color-bg, --color-text, etc.)
    - **Accessibility CSS variables:**
      - `--color-focus`: Focus outline color (must be high contrast)
      - `--outline-width`: Focus outline thickness
      - `--outline-offset`: Focus outline offset
    - Base styles all themes inherit
    - **Accessibility base styles:**
      - `.sr-only` class for screen reader only text
      - `:focus-visible` styles using CSS variables
      - `prefers-reduced-motion` media query
      - `forced-colors` media query support
    - Utility classes
  - Refactor all theme CSS files to:
    - Import base.css (if using @import) or define variables
    - Override only necessary variables
    - **Define accessible focus colors for each theme**
    - Consistent naming convention
  - Document all available CSS variables
  - Ensure themes can be switched by changing CSS file only
  - **Create `web/static/css/accessibility.css` with:**
    - Screen reader only utility class
    - Reduced motion support
    - High contrast mode support
    - Skip link styles

  **Must NOT do**:
  - Don't duplicate base styles in each theme

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: None
  - **Reason**: CSS architecture, refactoring, accessibility

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Tasks 13-17)
  - **Parallel Group**: After Wave 3 themes
  - **Blocks**: None
  - **Blocked By**: Tasks 13-17

  **References**:
  - Pattern: CSS custom properties for theming
  - Docs: CSS variables best practices
  - ARIA: WCAG contrast requirements

  **Acceptance Criteria**:
  - [ ] base.css created with common variables
  - [ ] All themes use consistent variables
  - [ ] Theme switching works by CSS change only
  - [ ] Documentation of variables
  - [ ] **Accessibility:**
    - [ ] Focus styles defined in all themes (highly visible)
    - [ ] `.sr-only` utility class works
    - [ ] `prefers-reduced-motion` respected
    - [ ] `forced-colors` media query present
    - [ ] accessibility.css created and included

  **QA Scenarios**:
  ```
  Scenario: Variable system works
    Tool: Bash
    Steps:
      1. Check all theme files use consistent variables
      2. Verify base.css is imported/included
      3. Test switching themes
    Expected: Consistent variable usage
    Evidence: .sisyphus/evidence/task-18-variables.txt
  
  Scenario: Accessibility utilities work
    Tool: Playwright
    Steps:
      1. Test .sr-only class hides visually but is in a11y tree
      2. Verify focus styles in all themes
      3. Test prefers-reduced-motion removes animations
    Expected: All a11y features work
    Evidence: .sisyphus/evidence/task-18-a11y-utils.png
  ```

  **Commit**: YES
  - Message: `refactor(themes): CSS variable system with accessibility`
  - Files: `web/static/themes/base.css`, `web/static/css/accessibility.css`, all theme files updated

---

- [ ] 19. End-to-End Integration (Backend + Frontend)

  **What to do**:
  - Wire up all components:
    - ALSA mixer changes trigger SSE broadcasts
    - Frontend receives SSE and updates UI via HTMX
    - Frontend user actions POST to backend
    - Backend updates ALSA and broadcasts to all clients
  - Implement polling loop for ALSA state changes:
    - Poll every 50-100ms when UI is connected
    - Compare current state with previous state
    - Only broadcast when values actually change
    - Stop polling when no clients connected (save CPU)
  - Add debouncing for rapid changes
  - Test with multiple browser tabs

  **Must NOT do**:
  - Don't poll ALSA when no clients connected
  - Don't broadcast if values haven't changed

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: None
  - **Reason**: Integration, concurrency, state management

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Waves 1-3)
  - **Parallel Group**: Wave 4 start
  - **Blocks**: Tasks 20-24
  - **Blocked By**: Waves 1-3

  **References**:
  - Pattern: Observer pattern for state changes
  - Code: `internal/server/` handlers need updates

  **Acceptance Criteria**:
  - [ ] Volume changes in UI update ALSA
  - [ ] ALSA changes (via amixer) update UI within 100ms
  - [ ] Multiple tabs stay synchronized
  - [ ] No unnecessary CPU usage when idle

  **QA Scenarios**:
  ```
  Scenario: Bidirectional sync
    Tool: Bash + Playwright
    Steps:
      1. Open UI in browser
      2. Change volume via amixer: `amixer set Master 50%`
      3. Verify UI updates within 100ms
      4. Change volume in UI
      5. Verify with `amixer get Master`
    Expected: Bidirectional sync works
    Evidence: .sisyphus/evidence/task-19-sync.mp4
  ```

  **Commit**: YES
  - Message: `feat: end-to-end integration`
  - Files: `internal/server/handlers.go`, `internal/alsa/monitor.go`

---

- [ ] 20. Network Interruption Recovery Logic

  **What to do**:
  - Implement client-side reconnection:
    - HTMX SSE extension auto-reconnects (built-in)
    - Add visual indicator when disconnected
    - Show "Reconnecting..." message
  - Server-side:
    - Handle client reconnections gracefully
    - Send full state on new connection (not just updates)
    - Clean up dead connections
  - Handle buffered changes (user requirement):
    - Client queues last state only (not history)
    - On reconnect, send current UI state to server
    - Server reconciles with ALSA
    - Discard old buffered values
  - Test scenarios:
    - WiFi disconnect/reconnect
    - Server restart
    - Browser sleep/wake

  **Must NOT do**:
  - Don't buffer thousands of changes
  - Only last state matters

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: None
  - **Reason**: Network resilience, error handling

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 19, 21-24)
  - **Blocks**: None
  - **Blocked By**: Task 19

  **References**:
  - Docs: HTMX SSE extension reconnection
  - Pattern: Last-write-wins reconciliation

  **Acceptance Criteria**:
  - [ ] Auto-reconnect works
  - [ ] Visual indicator shows connection status
  - [ ] State synchronized after reconnect
  - [ ] No duplicate/buffered stale values

  **QA Scenarios**:
  ```
  Scenario: Network interruption recovery
    Tool: Bash + Playwright
    Steps:
      1. Open UI, change volume to 50%
      2. Disconnect network (iptables or kill server)
      3. Change volume to 75% while offline
      4. Reconnect network
      5. Verify UI syncs to 75%
      6. Verify ALSA has 75%
    Expected: Recovery works, last state wins
    Evidence: .sisyphus/evidence/task-20-recovery.mp4
  ```

  **Commit**: YES
  - Message: `feat: network interruption recovery`
  - Files: `web/static/js/connection.js`, `internal/sse/client.go`

---

- [ ] 21. asound.conf Change Notification UI

  **What to do**:
  - UI component for notification banner:
    - Fixed position at top
    - Warning styling (yellow/orange)
    - Message: "Mixer configuration changed. Reload to update."
    - Reload button
    - Dismiss button
  - Integration with file watcher (Task 4):
    - Watcher triggers SSE event
    - Frontend displays banner
  - Banner persists until:
    - User clicks Reload
    - User dismisses (and it reappears on next change)
  - Handle multiple changes (debounce)

  **Must NOT do**:
  - Don't auto-reload (user might be mid-interaction)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: None
  - **Reason**: UI component, notification design

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 19-20, 22-24)
  - **Blocks**: None
  - **Blocked By**: Task 4

  **References**:
  - Pattern: Banner notifications
  - UX: Non-intrusive but visible

  **Acceptance Criteria**:
  - [ ] Banner appears when asound.conf changes
  - [ ] Reload button works
  - [ ] Dismiss button works
  - [ ] Styling consistent with theme

  **QA Scenarios**:
  ```
  Scenario: Config change notification
    Tool: Bash + Playwright
    Steps:
      1. Load UI
      2. Modify /etc/asound.conf (touch file)
      3. Wait 15 seconds
      4. Verify banner appears
      5. Click reload
      6. Verify page reloads
    Expected: Notification works
    Evidence: .sisyphus/evidence/task-21-notification.png
  ```

  **Commit**: YES
  - Message: `feat: asound.conf change notification`
  - Files: `web/templates/notification.html`, `web/static/css/notification.css`

---

- [ ] 22. Mobile Responsiveness Polish

  **What to do**:
  - Test all themes on mobile viewports:
    - iPhone SE (375x667)
    - iPhone 12 (390x844)
    - Android (360x640)
    - Tablet (768x1024)
  - Fix any layout issues:
    - Overflow/scrolling
    - Touch target sizes (min 44x44px)
    - Text readability
    - Slider usability
  - Ensure all themes have mobile variant:
    - Terminal: horizontal scroll acceptable
    - Modern: stack vertically
    - Muji: generous spacing
    - Mobile: optimized as designed
    - Creative: test usability
  - Add viewport meta tag if missing

  **Must NOT do**:
  - Don't sacrifice desktop for mobile (both should work)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: None
  - **Reason**: Responsive design, mobile UX

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 19-21, 23-24)
  - **Blocks**: None
  - **Blocked By**: Wave 3

  **References**:
  - Testing: Chrome DevTools device emulation
  - Standards: WCAG touch target guidelines

  **Acceptance Criteria**:
  - [ ] All themes usable on mobile
  - [ ] No horizontal scroll (except terminal)
  - [ ] Touch targets adequate size
  - [ ] Text readable without zoom

  **QA Scenarios**:
  ```
  Scenario: Mobile testing
    Tool: Playwright
    Steps:
      1. Test each theme at 375x667
      2. Verify controls accessible
      3. Test touch interactions
      4. Screenshot for each theme
    Expected: All themes mobile-friendly
    Evidence: .sisyphus/evidence/task-22-mobile/*.png
  ```

  **Commit**: GROUP with related theme fixes
  - Message: `fix(themes): mobile responsiveness`

---

- [ ] 23. Build System (Embed Static Files)

  **What to do**:
  - Set up Go embed for static files:
    - `//go:embed web/static/*` in embed.go
    - Serve embedded files via http.FileServer
  - Create build script (`scripts/build.sh`):
    - Build for multiple platforms (Linux amd64, arm64)
    - Strip debug symbols for smaller binary
    - Set version from git tag
  - Single binary should contain:
    - Go server code
    - All HTML templates
    - All CSS themes
    - HTMX library
    - No external file dependencies
  - Test binary works standalone:
    - Copy to /tmp
    - Run from there (no source files)
    - Verify all assets load

  **Must NOT do**:
  - Don't require source files at runtime
  - No external dependencies

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: None
  - **Reason**: Build automation, Go embed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 19-22, 24)
  - **Blocks**: Task 24
  - **Blocked By**: Waves 2-3

  **References**:
  - Docs: Go embed package
  - Pattern: Single binary deployment

  **Acceptance Criteria**:
  - [ ] Binary contains all static files
  - [ ] Binary runs standalone
  - [ ] Build script works
  - [ ] Multi-platform builds work

  **QA Scenarios**:
  ```
  Scenario: Standalone binary
    Tool: Bash
    Steps:
      1. Run `make build`
      2. Copy binary to /tmp/test/
      3. Run from /tmp/test/
      4. Verify UI loads with all themes
      5. Check no file system errors
    Expected: Fully self-contained
    Evidence: .sisyphus/evidence/task-23-standalone.txt
  ```

  **Commit**: YES
  - Message: `build: embed static files and build scripts`
  - Files: `internal/embed/embed.go`, `scripts/build.sh`, `Makefile`

---

- [ ] 24. Deployment Artifacts (Docker, Systemd)

  **What to do**:
  - Create `Dockerfile`:
    - Multi-stage build (build → runtime)
    - Based on scratch or alpine
    - Expose port 8080
    - Non-root user
  - Create `deploy/alsamixer-web.service` (systemd):
    - Service definition
    - Restart always
    - User/group configuration
  - Create `deploy/install.sh`:
    - Install binary to /usr/local/bin
    - Install systemd service
    - Create user if needed
  - Update README with deployment instructions
  - Test Docker image works
  - Test systemd service works

  **Must NOT do**:
  - Don't require privileged container
  - Don't run as root in container

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: None
  - **Reason**: DevOps, deployment

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 19-23)
  - **Blocks**: None
  - **Blocked By**: Task 23

  **References**:
  - Pattern: 12-factor app deployment
  - Docker: Multi-stage builds

  **Acceptance Criteria**:
  - [ ] Docker image builds and runs
  - [ ] Systemd service file valid
  - [ ] Install script works
  - [ ] Documentation complete

  **QA Scenarios**:
  ```
  Scenario: Docker deployment
    Tool: Bash
    Steps:
      1. Run `docker build -t alsamixer-web .`
      2. Run `docker run -p 8080:8080 alsamixer-web`
      3. Verify UI accessible
      4. Check container not running as root
    Expected: Docker deployment works
    Evidence: .sisyphus/evidence/task-24-docker.txt
  ```

  **Commit**: YES
  - Message: `deploy: Docker and systemd support`
  - Files: `Dockerfile`, `deploy/alsamixer-web.service`, `deploy/install.sh`

---

## Final Verification Wave (MANDATORY)

> 4 review agents run in PARALLEL. ALL must APPROVE. Rejection → fix → re-run.

- [ ] F1. **Plan Compliance Audit** — `oracle`

  **What to verify**:
  - All "Must Have" requirements implemented:
    - [ ] Real-time bidirectional sync works
    - [ ] All 5 themes functional
    - [ ] Mobile-responsive on all themes
    - [ ] asound.conf change detection
    - [ ] Network recovery works
    - [ ] Single binary deployment
    - [ ] **ARIA Accessibility: WCAG 2.1 AA compliance**
      - [ ] Semantic HTML structure
      - [ ] ARIA labels and roles on all controls
      - [ ] Keyboard navigation works (Tab, Arrows, Enter, Space)
      - [ ] Focus indicators visible on all themes
      - [ ] Screen reader compatible (tested with NVDA/VoiceOver)
      - [ ] High contrast mode support
  - All "Must NOT Have" guardrails respected:
    - [ ] NO authentication in code
    - [ ] NO HTTPS requirement
    - [ ] NO React/Vue/Angular
    - [ ] NO WebSockets
  - Evidence files exist for all tasks
  - No unaccounted files in git

  **Output**: `Must Have [7/7] | Must NOT Have [4/4] | Tasks [24/24] | VERDICT: APPROVE/REJECT`

  **Evidence**: `.sisyphus/evidence/F1-compliance-report.md`

---

- [ ] F2. **Code Quality Review** — `unspecified-high`

  **What to verify**:
  - [ ] `go vet ./...` passes
  - [ ] `gofmt -l .` returns no files
  - [ ] `go test ./...` passes
  - [ ] `go build` produces no warnings
  - [ ] No `panic()` in production code
  - [ ] No hardcoded credentials
  - [ ] Proper error handling throughout
  - [ ] Race detector: `go test -race ./...` passes
  - [ ] No AI slop patterns:
    - No excessive comments stating the obvious
    - No unused imports
    - No `as any` type assertions (Go: no interface{} abuse)
  - [ ] **Accessibility:**
    - [ ] axe DevTools: 0 violations
    - [ ] Lighthouse Accessibility: 95+ score
    - [ ] Semantic HTML validated
    - [ ] ARIA roles correctly implemented
    - [ ] CSS contrast ratios meet WCAG AA (4.5:1 for text, 3:1 for UI)

  **Output**: `Lint [PASS/FAIL] | Tests [N pass/N fail] | Race [PASS/FAIL] | A11y [PASS/FAIL] | Files [N clean/N issues] | VERDICT`

  **Evidence**: `.sisyphus/evidence/F2-quality-report.md`, `.sisyphus/evidence/F2-lighthouse-a11y.json`

---

- [ ] F3. **Real Hardware Testing** — `unspecified-high` (+ `playwright` skill)

  **What to verify**:
  - [ ] Test on real Linux system with ALSA
  - [ ] All mixer controls from `alsamixer` visible in web UI
  - [ ] Volume changes via UI affect `amixer get`
  - [ ] Volume changes via `amixer set` reflect in UI
  - [ ] Multiple browser tabs stay synchronized
  - [ ] Mobile browser (phone/tablet) works
  - [ ] Network disconnect/reconnect works
  - [ ] All 5 themes render correctly
  - [ ] asound.conf modification detected
  - [ ] Binary runs standalone (copied to /tmp)
  - [ ] **Accessibility testing:**
    - [ ] Keyboard-only navigation works (no mouse)
    - [ ] Screen reader (Orca on Linux) announces controls correctly
    - [ ] Focus indicators visible on all themes
    - [ ] axe DevTools: 0 critical/serious violations
    - [ ] High contrast mode usable (test with GNOME High Contrast)
    - [ ] 200% zoom level usable
    - [ ] Tab order logical (top-to-bottom, left-to-right)

  **Test Commands**:
  ```bash
  # Terminal 1: Start server
  ./alsamixer-web --port 8080

  # Terminal 2: Test ALSA integration
  amixer set Master 50%
  amixer set Master 75%
  amixer set Master mute
  amixer set Master unmute

  # Terminal 3: Test keyboard navigation
  # Open browser, navigate entirely with Tab/Arrow/Enter keys

  # Terminal 4: Test with screen reader
  orca &  # Enable Orca screen reader
  # Navigate UI and verify announcements

  # Browser: Open UI, verify changes reflected
  # Run axe DevTools extension
  # Run Lighthouse accessibility audit
  ```

  **Output**: `Tests [16/16 pass] | VERDICT`

  **Evidence**: Screenshots and screen recordings in `.sisyphus/evidence/F3-hardware-testing/`

---

- [ ] F4. **Scope Fidelity Check** — `deep`

  **What to verify**:
  - Review each task "What to do" vs actual implementation
  - Verify 1:1 correspondence:
    - [ ] Everything specified was built
    - [ ] Nothing extra was built (no scope creep)
  - Check "Must NOT do" compliance for each task
  - Detect cross-task contamination:
    - [ ] Task N doesn't modify Task M's files unnecessarily
  - Flag any unaccounted changes
  - Verify plan architecture matches implementation

  **Output**: `Tasks [24/24 compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

  **Evidence**: `.sisyphus/evidence/F4-fidelity-report.md`

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 1 | `chore: initial project structure` | All new files | `go build ./...` |
| 2 | `feat(alsa): mixer abstraction layer` | `internal/alsa/*` | `go test ./internal/alsa/...` |
| 3 | `feat(sse): broadcast hub implementation` | `internal/sse/*` | `go test -race ./internal/sse/...` |
| 4 | `feat(config): asound.conf file watcher` | `internal/config/watcher.go` | `go test ./internal/config/...` |
| 5 | `feat(server): HTTP server scaffolding` | `internal/server/*`, `web/static/` | `curl http://localhost:8080/` |
| 6 | `feat(config): CLI flags and environment variables` | `internal/config/config.go`, `cmd/*/main.go` | `./alsamixer-web --help` |
| 7 | `feat(web): HTMX and SSE integration` | `web/static/js/`, `web/templates/base.html` | Browser console check |
| 8 | `feat(web): basic control templates` | `web/templates/controls.html`, `internal/server/handlers.go` | Visual verification |
| 9 | `feat(theme): terminal/ASCII volume slider` | `web/static/themes/terminal.css` | Screenshot |
| 10 | `feat(theme): mute and capture toggles` | `web/static/themes/terminal.css` | Interaction test |
| 12 | `feat(web): theme selector` | `web/templates/base.html`, `internal/server/handlers.go` | Theme switching test |
| 13 | `feat(theme): complete terminal/ASCII theme` | `web/static/themes/terminal.css` | Compare to alsamixer |
| 14 | `feat(theme): modern clean theme` | `web/static/themes/modern.css` | Screenshot |
| 15 | `feat(theme): Muji design theme` | `web/static/themes/muji.css` | Screenshot |
| 16 | `feat(theme): mobile-first theme` | `web/static/themes/mobile.css`, `web/static/js/mobile.js` | Mobile test |
| 17 | `feat(theme): creative theme` | `web/static/themes/creative.css` | Screenshot |
| 18 | `refactor(themes): CSS variable system` | `web/static/themes/base.css`, theme updates | Consistency check |
| 19 | `feat: end-to-end integration` | `internal/server/handlers.go`, `internal/alsa/monitor.go` | Bidirectional sync test |
| 20 | `feat: network interruption recovery` | `web/static/js/connection.js`, `internal/sse/client.go` | Disconnect/reconnect test |
| 21 | `feat: asound.conf change notification` | `web/templates/notification.html`, `web/static/css/notification.css` | File touch test |
| 23 | `build: embed static files and build scripts` | `internal/embed/embed.go`, `scripts/build.sh`, `Makefile` | Standalone binary test |
| 24 | `deploy: Docker and systemd support` | `Dockerfile`, `deploy/*` | Docker build test |

---

## Success Criteria

### Verification Commands

```bash
# Build
go build -o alsamixer-web ./cmd/alsamixer-web

# Test
go test -race ./...

# Quality
go vet ./...
gofmt -l .

# Integration test
./alsamixer-web --port 8080 &
curl -s http://localhost:8080/ | head -20
curl -s http://localhost:8080/events &
amixer set Master 50%
# Verify SSE event received

# Docker
docker build -t alsamixer-web .
docker run -p 8080:8080 alsamixer-web
```

### Final Checklist

- [ ] All 24 tasks completed
- [ ] All 4 verification agents approve
- [ ] Binary builds without errors
- [ ] All 5 themes render correctly
- [ ] Real-time sync works (<100ms latency)
- [ ] Network recovery works
- [ ] Mobile-responsive on all themes
- [ ] asound.conf change detection works
- [ ] No authentication in code
- [ ] No HTTPS requirement
- [ ] Single binary deployment works
- [ ] Docker image builds and runs
- [ ] Documentation complete
- [ ] **ARIA Accessibility: WCAG 2.1 AA compliance**
  - [ ] axe DevTools: 0 critical/serious violations
  - [ ] Lighthouse Accessibility: 95+ score
  - [ ] Keyboard navigation works (tested)
  - [ ] Screen reader compatible (tested with Orca/NVDA)
  - [ ] Focus indicators visible on all themes
  - [ ] High contrast mode supported

### Deliverables Checklist

- [ ] `alsamixer-web` binary (Linux amd64, arm64)
- [ ] `Dockerfile` for containerized deployment
- [ ] `deploy/alsamixer-web.service` for systemd
- [ ] `deploy/install.sh` installation script
- [ ] `README.md` with usage instructions
- [ ] 5 theme CSS files in `web/static/themes/`
- [ ] All evidence files in `.sisyphus/evidence/`

---

## ARIA Accessibility Requirements

### Accessibility Standards
Target: **WCAG 2.1 Level AA** compliance

### Key Requirements

#### 1. Semantic HTML Structure
- Use semantic elements: `<main>`, `<section>`, `<article>`, `<button>`, `<input>`
- Avoid generic `<div>` soup for interactive elements
- Proper heading hierarchy (`<h1>` for page title, `<h2>` for cards)

#### 2. ARIA Labels and Roles

**Control Components:**
```html
<!-- Volume Slider -->
<div role="slider" 
     aria-label="Master volume"
     aria-valuemin="0" 
     aria-valuemax="100" 
     aria-valuenow="75"
     aria-valuetext="75 percent"
     tabindex="0">
</div>

<!-- Mute Toggle -->
<button role="switch" 
        aria-label="Master mute"
        aria-checked="false"
        type="button">
  <span aria-hidden="true">M</span>
</button>

<!-- Capture Toggle -->
<button role="switch" 
        aria-label="Master capture"
        aria-checked="true"
        type="button">
  <span aria-hidden="true">C</span>
</button>
```

**Card Structure:**
```html
<section aria-labelledby="card-0-title">
  <h2 id="card-0-title">Card 0: bcm2835 ALSA</h2>
  <!-- controls -->
</section>
```

#### 3. Keyboard Navigation
- **Tab**: Move between controls
- **Arrow Keys**: Adjust volume (up/down/left/right)
- **Enter/Space**: Toggle mute/capture
- **Home/End**: Min/max volume
- **Page Up/Down**: Large volume steps (10%)

**Focus Management:**
- Visible focus indicators on all themes
- Focus trap within modals (if any)
- Skip links for main content

#### 4. Focus Indicators (All Themes)

```css
/* Ensure focus visible on ALL themes */
:focus-visible {
  outline: 3px solid var(--color-focus);
  outline-offset: 2px;
}

/* Terminal theme - high contrast */
.terminal :focus-visible {
  outline: 2px solid #0f0;
  background: rgba(0, 255, 0, 0.2);
}

/* Modern theme - accent color */
.modern :focus-visible {
  outline: 3px solid var(--color-accent);
  outline-offset: 3px;
}
```

#### 5. ARIA Live Regions for SSE Updates

Screen readers must announce volume changes from SSE:

```html
<!-- Hidden live region for announcements -->
<div id="announcer" 
     aria-live="polite" 
     aria-atomic="true"
     class="sr-only">
</div>
```

```javascript
// When SSE updates volume
function announceVolumeChange(control, value) {
  const announcer = document.getElementById('announcer');
  announcer.textContent = `${control} volume changed to ${value} percent`;
}
```

#### 6. Screen Reader Only Content

```css
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
```

#### 7. High Contrast Support

```css
/* Respect Windows High Contrast Mode */
@media (forced-colors: active) {
  .volume-fill {
    forced-color-adjust: auto;
  }
  
  :focus-visible {
    outline: 3px solid CanvasText;
  }
}

/* Respect macOS Increase Contrast */
@media (prefers-contrast: more) {
  .control {
    border: 2px solid currentColor;
  }
}
```

#### 8. Reduced Motion

```css
@media (prefers-reduced-motion: reduce) {
  * {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

### Accessibility Testing Checklist

#### Automated Testing
- [ ] axe DevTools: No violations
- [ ] Lighthouse Accessibility: 100 score
- [ ] WAVE: No errors
- [ ] Pa11y: No issues

#### Manual Testing
- [ ] Keyboard-only navigation works
- [ ] Screen reader (NVDA/VoiceOver) announces correctly
- [ ] Focus indicators visible on all themes
- [ ] High contrast mode usable
- [ ] 200% zoom usable

### Screen Reader Testing Commands

```bash
# Test with NVDA (Windows)
# 1. Install NVDA
# 2. Navigate to http://localhost:8080
# 3. Press Tab to navigate
# 4. Verify announcements:
#    - "Master volume, slider, 75"
#    - "Mute, switch, not checked"

# Test with VoiceOver (macOS)
# Cmd + F5 to enable
# Ctrl + Option + Arrow keys to navigate

# Test with Orca (Linux)
# Install: sudo apt install orca
# Enable: Super + Alt + S
```

---

## Theme Reference

### Terminal/ASCII Theme

Reference: Classic ncurses alsamixer appearance
- Monospace font (Courier New, Consolas, or system monospace)
- Box drawing characters for borders (┌─┐│└┘) or CSS borders
- Block characters for volume bars (█, ░)
- Dark background (#000 or #1a1a1a)
- Light text (#0f0 green, or #fff white)
- [M] for mute, [C] for capture indicators
- Card headers with device name
- Vertical or horizontal control layout

Visual reference: https://upload.wikimedia.org/wikipedia/commons/8/86/Alsamixer.png

Key elements to replicate:
1. Box-drawn card containers with titles
2. Volume bars showing percentage fill with block characters
3. Numeric volume values displayed
4. Mute [M] and Capture [C] toggle indicators
5. Arrow key navigation hints at bottom (optional for web)
6. Clean grid-like layout

### Modern Clean Theme

- White/light gray background
- Subtle shadows and rounded corners (4-8px radius)
- System sans-serif font
- Clean range inputs for sliders
- Toggle switches for mute/capture
- Card-based layout with whitespace

### Muji Theme

- Beige/off-white background (#F5F5DC, #FFFEF0)
- Brown/earth tone accents (#8B7355)
- Minimal borders (use spacing instead)
- Natural, calm aesthetic
- Generous padding
- Simple, unobtrusive controls

### Mobile-First Theme

- Full-width cards
- Single control per panel (swipe between)
- Large touch targets (44x44px minimum)
- Bottom navigation
- Dot indicators for position
- Vertical volume sliders preferred
- Large, readable text

### Creative Theme

- Distinctive, bold aesthetic
- Could use:
  - Neon accents on dark background
  - Glassmorphism (frosted glass effect)
  - Retro/synthwave colors
  - Brutalist design
- Must remain functional and usable
- Should be memorable and different

---

## Technical Notes for Implementation

### ALSA Polling Strategy

```go
// Pseudo-code for efficient polling
func (m *Mixer) Monitor() {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    var lastState State
    for range ticker.C {
        if !m.hasClients() {
            continue // Skip if no clients connected
        }
        
        currentState := m.GetState()
        if !currentState.Equals(lastState) {
            m.hub.Broadcast(currentState)
            lastState = currentState
        }
    }
}
```

### SSE Event Format

```
event: volume-change
data: {"card": 0, "control": "Master", "values": [50, 50]}

event: mute-change
data: {"card": 0, "control": "Master", "muted": false}

event: config-changed
data: {"file": "/etc/asound.conf", "timestamp": "2024-01-01T00:00:00Z"}
```

### HTMX Attributes Reference

```html
<!-- SSE connection -->
<div hx-ext="sse" sse-connect="/events">
  
  <!-- Update on specific event -->
  <div sse-swap="volume-change" hx-target="this">
    <!-- Server pushes updated HTML here -->
  </div>
  
  <!-- POST on user interaction -->
  <input type="range" 
         hx-post="/control/volume" 
         hx-vals='{"card": 0, "control": "Master"}'
         hx-trigger="change">

</div>
```

### CSS Variable Structure

```css
/* base.css */
:root {
  /* Colors */
  --color-bg: #ffffff;
  --color-text: #333333;
  --color-accent: #2196f3;
  --color-muted: #999999;
  
  /* Spacing */
  --spacing-xs: 0.25rem;
  --spacing-sm: 0.5rem;
  --spacing-md: 1rem;
  --spacing-lg: 2rem;
  
  /* Typography */
  --font-family: system-ui, sans-serif;
  --font-size-sm: 0.875rem;
  --font-size-md: 1rem;
  --font-size-lg: 1.25rem;
  
  /* Borders */
  --border-radius: 4px;
  --border-width: 1px;
  
  /* Shadows */
  --shadow-sm: 0 1px 2px rgba(0,0,0,0.1);
  --shadow-md: 0 2px 4px rgba(0,0,0,0.1);
}
```

---

## Development & Testing Environment

### macOS Development Limitation

Since development is performed on macOS, direct testing of the **BFF ↔ ALSA Linux sound subsystem** interaction is not possible. ALSA (Advanced Linux Sound Architecture) is Linux-specific and requires:
- Linux kernel with ALSA support
- ALSA library (`libasound2`)
- Physical or virtual sound hardware

### Remote Test System: lemox

A dedicated Linux test system is available for remote testing and validation:

**Connection Details:**
- **Host**: `lemox.lan` (accessible via SSH)
- **Project Directory**: `/root/Development/alsamixer-web/`
- **Access**: SSH + SCP for file transfer

**Usage Patterns:**

```bash
# Copy built binary to remote system
scp ./alsamixer-web lemox.lan:/root/Development/alsamixer-web/

# Run tests on remote Linux system with ALSA
ssh lemox.lan 'cd /root/Development/alsamixer-web && ./alsamixer-web --port 8080'

# Check remote directory structure
ssh lemox.lan 'find /root/Development/alsamixer-web/ -type f | head -20'

# Run diagnostic commands
ssh lemox.lan 'amixer scontrols'  # List available mixer controls
ssh lemox.lan 'amixer get Master' # Get Master volume
```

**Important Constraints:**

1. **STRICT Project Directory Policy**: 
   - ALL file operations MUST stay within `/root/Development/alsamixer-web/`
   - NO modifications outside this directory
   - NO system-wide installations without explicit permission

2. **Testing Workflow**:
   - Develop locally on macOS
   - Build binary locally (`make build`)
   - Copy binary to lemox via SCP
   - Test ALSA integration on lemox
   - Debug via SSH remote execution
   - UI testing via Playwright (can connect to lemox.lan:8080)

3. **E2E Testing via Playwright**:
   ```javascript
   // Connect Playwright to remote instance
   await page.goto('http://lemox.lan:8080');
   // Perform UI interactions and assertions
   ```

**Testing Checklist (on lemox):**
- [ ] Binary runs without errors
- [ ] ALSA mixer controls enumerated correctly
- [ ] Volume changes via UI reflect in `amixer get`
- [ ] Volume changes via `amixer set` reflect in UI
- [ ] SSE events broadcast to connected clients
- [ ] All 5 themes render correctly
- [ ] Accessibility features work (keyboard nav, screen reader)

---

**Plan Version**: 1.0
**Created**: 2026-02-18
**Status**: Ready for execution

**Next Step**: Run `/start-work` to begin execution with Sisyphus
