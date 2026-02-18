## Decisions
- Use Go's flag package for CLI parsing and override order: flags > env vars > defaults.
- Implement a HelpText() helper to expose usage without running main.
- Keep configuration in a single internal/config package to minimize dependencies.
