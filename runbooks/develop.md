# Working on tuki

Go 1.24 or newer. No other tooling required — the Charm libraries come in via
modules, and there's nothing to generate.

```sh
git clone https://github.com/rmpato/tuki
cd tuki
make check      # gofmt, go vet, go test ./...
make build      # ./tuki
```

## Run it without touching your own tasks

```sh
TUKI_HOME=$(mktemp -d) go run .
```

`TUKI_FILE` (exact file), `TUKI_HOME` (directory), and `XDG_DATA_HOME` are
checked in that order, so any of them will do. `tuki path` prints where it
landed.

## Layout

| package | what lives there |
| --- | --- |
| `internal/task` | the domain: a task, its tag, date parsing and formatting |
| `internal/store` | the JSON file, loaded and saved atomically |
| `internal/ui` | palette, styles, faces, and the lines tuki says |
| `internal/tui` | the full-screen interface (Bubble Tea) |
| `internal/cli` | the commands (Cobra, wrapped in Fang) |
| `internal/update` | the self-updater |
| `docs/` | the website — see [website.md](website.md) |

`internal/task` deliberately imports nothing but the standard library, which
keeps the interesting logic cheap to test.

## Tests

```sh
go test ./...
go test -race ./...
go test ./internal/tui -run TestFrameFits -v
```

The TUI tests drive the real `Update`/`View` loop headlessly and assert on
frames with the styling stripped, so layout regressions fail the build rather
than showing up in someone's terminal. `TestFrameFitsTerminal` checks eight
states against five terminal sizes down to 20×6.

Two things worth knowing if you touch that suite:

- **Commands are run with a timeout.** Several are `tea.Tick` timers that
  would otherwise sleep for seconds. The assertions are all about state that
  `Update` sets synchronously.
- **Key names come from Bubble Tea v2.** Space is `"space"`, not `" "`.

## Debugging the interface

You can't print to stdout while the TUI owns the screen. Log to a file:

```go
f, _ := tea.LogToFile("/tmp/tuki.log", "tuki")
defer f.Close()
```

```sh
tail -f /tmp/tuki.log
```

## Conventions

- `make check` before pushing; CI runs the same three things on Linux and
  macOS and will fail on unformatted code.
- The narrow column, the whitespace, and the quietness are the product. New
  output should earn its line.
- No streaks, points, scores, or notifications. That's the one hard rule.
