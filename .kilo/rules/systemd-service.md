# Systemd Service Management for alsamixer-web

## Service Type
- **USER service**, NOT a system service
- Service file lives in the app directory: `/root/work/alsamixer-web/alsamixer-web.service`
- Symlinked to user systemd: `/root/.config/systemd/user/alsamixer-web.service`

## Service Management Commands
All commands use `--user` flag:
```bash
# Status
systemctl --user status alsamixer-web.service

# Start/Stop/Restart
systemctl --user start alsamixer-web.service
systemctl --user stop alsamixer-web.service
systemctl --user restart alsamixer-web.service

# Enable/Disable
systemctl --user enable alsamixer-web.service
systemctl --user disable alsamixer-web.service

# Logs
journalctl --user-unit alsamixer-web -n 50 --no-pager

# Reload daemon after unit file changes
systemctl --user daemon-reload
```

## NEVER Do These
1. **NEVER** install to `/etc/systemd/system/` (system service)
2. **NEVER** use `systemctl` without `--user` flag
3. **NEVER** manually start the binary on lemox - use the systemd service

## Deployment Flow
1. Atomic deploy: `scp` to `.new` then `mv` (avoids "text file busy")
2. Wrapper detects inode change and exits with code 2
3. systemd restarts automatically via `Restart=on-failure`
4. No manual service restart needed during deploy

## Wrapper Script
The `alsamixer-web-wrapper` handles:
- Monitoring binary for changes (inode-based detection)
- Exit codes: 0=clean, 1=crash, 2=binary updated
- Letting systemd handle all restarts
