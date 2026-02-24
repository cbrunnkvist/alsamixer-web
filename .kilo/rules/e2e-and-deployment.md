# E2E Testing, Deployment, Starting alsamixer-web, Checking Logs on "lemox.lan"

## E2E Testing

### Running Tests
- Always use the remote server `lemox.lan` for E2E tests, not localhost
- Required environment variables:
  - `E2E_BASE_URL=http://lemox.lan:8888`
  - `E2E_SERVER_CMD_PREFIX=ssh lemox.lan` (for ALSA commands)

### Test Commands
```bash
# Basic UI tests
E2E_BASE_URL=http://lemox.lan:8888 E2E_SERVER_CMD_PREFIX="ssh lemox.lan" node e2e.test.js

# ALSA→UI sync tests
E2E_BASE_URL=http://lemox.lan:8888 E2E_SERVER_CMD_PREFIX="ssh lemox.lan" node e2e-alsa-to-ui.test.js

# Mute toggle tests
E2E_BASE_URL=http://lemox.lan:8888 E2E_SERVER_CMD_PREFIX="ssh lemox.lan" node e2e-mute.test.js
```

### Browser Rules
- **NEVER** run `browser_install` - use existing Brave browser
- Brave path: `/Applications/Brave Browser.app/Contents/MacOS/Brave Browser`
- Tests run in non-headless mode by default

## Deployment

### Deployment Server
- Server: `lemox.lan`
- Deployment path: `/root/work/alsamixer-web`

### Service Management
**See `.kilo/rules/systemd-service.md` for complete systemd documentation.**

Key points:
- Service is a **user service** (use `systemctl --user`)
- Service file: `/root/work/alsamixer-web/alsamixer-web.service` (symlinked to user systemd)
- Logs: `journalctl --user-unit alsamixer-web`
- Status: `systemctl --user status alsamixer-web.service`

### Deployment Commands
```bash
# Build and deploy (atomic - uses mv for binary replacement)
make deploy DEPLOY_TARGET=root@lemox.lan DEPLOY_PATH=/root/work/alsamixer-web

# The wrapper script handles automatic restart when binary changes
# No manual service restart needed
```

