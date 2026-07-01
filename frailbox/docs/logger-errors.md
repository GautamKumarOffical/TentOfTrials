# Legacy logger error handling

The frailbox legacy logger (`src/logger.c`) writes to stderr by default or to the path in `LOG_FILE`.

## Open failures

When `LOG_FILE` is set but cannot be opened, the logger:

1. Prints a single stderr diagnostic with the path and a human-readable reason (`permission denied`, `parent directory or file path missing`, etc.).
2. Falls back to stderr for all subsequent log lines.
3. Records the message via `log_last_io_error()` for tests and troubleshooting.

Logging continues after fallback; messages are not dropped solely because the file could not be opened.

## Write / flush failures

If writing or flushing to the configured file fails, the logger:

1. Emits a stderr diagnostic describing the failure.
2. Retries the same formatted line on stderr when possible.
3. Marks stderr fallback as active through `log_uses_stderr_fallback()`.

## Testing

```bash
cd frailbox
make test-logger-errors
```

The harness verifies invalid `LOG_FILE` paths still allow log output and expose fallback state through the public helpers.
<!-- LEGACY: frailbox/docs/logger-errors.md -->
