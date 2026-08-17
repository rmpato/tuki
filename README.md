# tuki

A tiny terminal companion that remembers the things you need to do.

**[rmpato.github.io/tuki](https://rmpato.github.io/tuki/)** — try the interface in your browser.

`tuki` is a todo app that fits in one small window, keeps everything in one
local file, and has opinions about roughly nothing. No accounts, no sync, no
streaks, no points, no notifications. You type `tuki`, your tasks are there.

```
   (o.o)  tuki                            ━━━━━━━━━━━━  3/7 done


   WORK ──────────────────────────────────────────────── 1/3

     ✓  ship the release notes
   ▸ ○  write the docs                                    fri
     ○  reply to sam                                   3d ago

   HOME ──────────────────────────────────────────────── 1/2

     ○  buy oat milk                                 tomorrow
     ✓  water the plants

   HOBBY ─────────────────────────────────────────────── 1/2

     ○  learn one more chord
     ✓  finish the zine


   ↑/k up • ↓/j down • space done • a add • d delete • ? keys • q quit
```

Completed things stay where they are, crossed out and faded, until you clear
them. Finishing one gets you a brief, quiet reaction and nothing more.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/rmpato/tuki/main/install.sh | sh
```

That fetches the right build for your machine, checks it against the release's
sha256, installs it to `~/.local/bin`, and adds that directory to your `PATH`
if it isn't already there. Pass `--dir <path>` to install somewhere else, or
`--no-modify-path` to leave your shell config alone.

<details>
<summary>Other ways</summary>

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
[releases page](https://github.com/rmpato/tuki/releases).

</details>

### Keeping it current

```sh
tuki update
```

tuki checks GitHub, shows you what's new, and asks before replacing anything.
The download is verified against the release checksum, and your current binary
is kept aside until the new one is safely in place. `tuki update --check` just
reports; `tuki update --yes` skips the question, for scripts.

## The interface

Running `tuki` with no arguments opens the full-screen interface.

| key            | does                                        |
| -------------- | ------------------------------------------- |
| `↑` `↓` `k` `j`| move                                        |
| `g` `G`        | jump to the top / bottom                    |
| `space`        | complete or un-complete                     |
| `a`            | add a task                                  |
| `e`            | edit the selected task                      |
| `d`            | delete it                                   |
| `u`            | undo the last delete or clear               |
| `t`            | cycle its group                             |
| `D`            | set a due date                              |
| `tab` `←` `→`  | show one group at a time                    |
| `/`            | search                                      |
| `c`            | clear completed tasks                       |
| `?`            | show every key                              |
| `q`            | leave                                       |

When you add or edit a task you can write the group and the due date inline:

```
buy oat milk #home @tomorrow
```

The editor stays open after each `enter`, so several tasks in a row is one
uninterrupted motion.

## The commands

Everything is also available as a plain command, so `tuki` works in scripts and
pipes:

```sh
tuki add "buy oat milk" --tag home --due tomorrow
tuki add "write the docs #work @fri"
tuki list
tuki list --tag work --todo
tuki done 12
tuki undone 12
tuki edit 12 "write better docs"
tuki edit 12 --due none
tuki rm 12
tuki clear
tuki tags
tuki path
```

| command  | does                                        |
| -------- | ------------------------------------------- |
| `add`    | remember something new                      |
| `list`   | show your tasks (`ls`)                      |
| `done`   | cross something off                         |
| `undone` | put it back on the list                     |
| `edit`   | reword it, or change its group or due date  |
| `rm`     | forget it entirely                          |
| `clear`  | remove completed tasks (`--all` for all)    |
| `tags`   | list the groups you're using                |
| `path`   | print where your tasks live                 |
| `update` | check for a newer tuki and install it       |

### In scripts

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
interface nobody can see.

## Groups and due dates

Groups are just tags. `work`, `home`, `hobby`, and `misc` come with reserved
colours and a reserved spot in the order; anything else you invent works too
and gets a colour of its own. A task without a tag is `misc`.

Due dates are written the way you'd say them:

```
today   tomorrow   yesterday   fri   next week
+3d     2w         1m          2026-08-20      08-20
none
```

`tuki` only mentions a due date when it's worth mentioning, and only comments
on overdue tasks once there are a few of them.

## Where things live

One JSON file, by default:

```
~/.local/share/tuki/tasks.json
```

`tuki path` prints it. It's plain JSON, written atomically, and safe to read,
back up, edit, or delete. To put it somewhere else, set `TUKI_FILE` to an exact
path, `TUKI_HOME` to a directory, or `XDG_DATA_HOME` — checked in that order.

`TUKI_THEME=light` or `dark` overrides the background-colour guess used by the
one-shot commands. The full-screen interface asks your terminal directly.

## Turning tuki down

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

## Built with

[Bubble Tea](https://github.com/charmbracelet/bubbletea) for the interface,
[Lip Gloss](https://github.com/charmbracelet/lipgloss) for the styling,
[Bubbles](https://github.com/charmbracelet/bubbles) for the input, viewport,
help, and progress bar, and [Fang](https://github.com/charmbracelet/fang) for
the command line — all from [Charm](https://charm.sh).

## Development

```sh
make test    # go test ./...
make build   # build ./tuki
make check   # fmt, vet, and test
```

## License

MIT
