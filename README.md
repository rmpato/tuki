# tuki

A tiny terminal companion that remembers the things you need to do. One local
file. No accounts, no sync, no streaks, no points, no notifications. You type
`tuki`, your tasks are there.

**[Try the interface in your browser →](https://rmpato.github.io/tuki/)**

![tuki running in a terminal, with tasks grouped under WORK, HOME, HOBBY and
MISC, completed ones crossed out, and a 3/8 done counter](docs/screenshot.png)

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/rmpato/tuki/main/install.sh | sh
```

Checks the release's sha256, installs to `~/.local/bin`, and puts that on your
`PATH`. Takes `--dir <path>` and `--no-modify-path`. Go, source, and prebuilt
binaries are under [Details](#details).

## Start

```sh
tuki add "buy oat milk #home @tomorrow"   # remember something
tuki                                      # open the interface
tuki done 1                               # or stay on the command line
```

Completed tasks stay where they are, crossed out and faded, until you clear
them. Finishing one gets you a brief, quiet reaction and nothing more.

### Keys

`tuki` with no arguments opens the full-screen interface.

| key             | does                          |
| --------------- | ----------------------------- |
| `↑` `↓` `k` `j` | move                          |
| `g` `G`         | jump to the top / bottom      |
| `space`         | complete or un-complete       |
| `a`             | add a task                    |
| `e`             | edit the selected task        |
| `d`             | delete it                     |
| `u`             | undo the last delete or clear |
| `t`             | cycle its group               |
| `D`             | set a due date                |
| `tab` `←` `→`   | show one group at a time      |
| `/`             | search                        |
| `c`             | clear completed tasks         |
| `?`             | show every key                |
| `q`             | leave                         |

Adding and editing accept `#tag` and `@when` inline — `@fri`, `@+3d`,
`@2026-08-20`. The add field stays open after each `enter`, so several tasks in
a row is one uninterrupted motion.

### Commands

Everything is also a plain command, so tuki works in scripts and pipes.

| command            | does                                                              |
| ------------------ | ----------------------------------------------------------------- |
| `add <text>`       | remember something new — `--tag`, `--due`                         |
| `list` (`ls`)      | show your tasks — `--tag`, `--todo`, `--done`, `--json`, `--plain` |
| `done <id>...`     | cross something off                                               |
| `undone <id>...`   | put it back on the list                                           |
| `edit <id> [text]` | reword it, or change its group or due date — `--tag`, `--due`     |
| `rm <id>...`       | forget it entirely                                                |
| `clear`            | remove completed tasks — `--all` for everything, `--yes` to skip the prompt |
| `tags`             | list the groups you're using                                      |
| `path`             | print where your tasks live                                       |
| `update`           | check for a newer tuki and install it — `--check`, `--yes`        |

`--file <path>` works on any of them, and `tuki --help` brings shell
completions and a manpage with it.

## Details

<details>
<summary><b>Groups and due dates</b> — the tags, and every date format</summary>

Groups are just tags. `work`, `home`, `hobby`, and `misc` have reserved colours
and a reserved spot in the order; anything else you invent works too and gets a
colour of its own. A task without a tag is `misc`.

Due dates are written the way you'd say them:

```
today   tomorrow   yesterday   fri   next week
+3d     2w         1m          2026-08-20   08-20   none
```

tuki only mentions a due date when it's worth mentioning, and only comments on
overdue tasks once there are a few of them.

</details>

<details>
<summary><b>In scripts</b> — tab-separated output, JSON, exit behaviour</summary>

When output isn't a terminal, `tuki list` switches to tab-separated columns:

```
id	done|todo	tag	due	text
```

So the usual things work:

```sh
tuki list | awk -F'\t' '$2 == "todo" { print $5 }'
tuki list --json | jq '.[] | select(.tag == "work")'
tuki list --todo --tag work | wc -l
```

`tuki` on its own does the same thing when piped, rather than opening an
interface nobody can see. `--plain` forces the script format even on a
terminal.

</details>

<details>
<summary><b>Where your tasks live</b> — the file, and how to move it</summary>

One JSON file, by default:

```
~/.local/share/tuki/tasks.json
```

`tuki path` prints it. It's plain JSON, written atomically, and safe to read,
back up, edit, or delete. To put it somewhere else, set `TUKI_FILE` to an exact
path, `TUKI_HOME` to a directory, or `XDG_DATA_HOME` — checked in that order.

`TUKI_THEME=light` or `dark` overrides the background-colour guess used by the
one-shot commands. The full-screen interface asks your terminal directly.

</details>

<details>
<summary><b>Turning tuki down</b> — the only two switches</summary>

The two bits of personality live in the settings block of your tasks file:

```json
"settings": {
  "celebrate": true,
  "judge": true
}
```

Set `celebrate` to `false` for no reaction when you complete something, and
`judge` to `false` if you'd rather tuki didn't mention the overdue pile. There
is nothing else to turn off, because there is nothing else.

</details>

<details>
<summary><b>Other ways to install</b> — Go, source, prebuilt binaries</summary>

With Go:

```sh
go install github.com/rmpato/tuki@latest
```

This puts `tuki` in `$(go env GOPATH)/bin` — usually `~/go/bin` — which isn't
on your `PATH` by default. Add it:

```sh
export PATH="$HOME/go/bin:$PATH"
```

From a clone:

```sh
git clone https://github.com/rmpato/tuki
cd tuki
make install
```

Or grab a binary straight from the
[releases page](https://github.com/rmpato/tuki/releases) — macOS, Linux, and
Windows, on amd64 and arm64.

</details>

<details>
<summary><b>Updating</b> — what <code>tuki update</code> actually does</summary>

```sh
tuki update
```

tuki checks GitHub, shows you what's new, and asks before replacing anything.
The download is verified against the release checksum, and your current binary
is kept aside until the new one is safely in place, so a failed swap rolls
back. `tuki update --check` only reports; `tuki update --yes` skips the
question, for scripts.

</details>

<details>
<summary><b>Building it</b> — the make targets</summary>

```sh
make test    # go test ./...
make build   # build ./tuki
make check   # fmt, vet, and test
```

[`runbooks/`](runbooks/) has step-by-step for cutting a release, redeploying
the site, regenerating the screenshots (`./tools/screenshots.sh`), and finding
your way around the code.

</details>

## Built with

[Bubble Tea](https://github.com/charmbracelet/bubbletea) for the interface,
[Lip Gloss](https://github.com/charmbracelet/lipgloss) for the styling,
[Bubbles](https://github.com/charmbracelet/bubbles) for the input, viewport,
help, and progress bar, and [Fang](https://github.com/charmbracelet/fang) for
the command line — all from [Charm](https://charm.sh).

MIT
