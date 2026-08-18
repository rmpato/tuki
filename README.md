# tuki

[![ci](https://github.com/rmpato/tuki/actions/workflows/ci.yml/badge.svg)](https://github.com/rmpato/tuki/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/rmpato/tuki?color=b4852f&label=release)](https://github.com/rmpato/tuki/releases)
[![go report](https://goreportcard.com/badge/github.com/rmpato/tuki)](https://goreportcard.com/report/github.com/rmpato/tuki)
[![license](https://img.shields.io/badge/license-MIT-b4852f)](LICENSE)

> What do I need to do?

A tiny terminal companion that remembers the things you need to do. One local
JSON file. No accounts, no sync, no streaks, no points, no notifications.

**[rmpato.github.io/tuki](https://rmpato.github.io/tuki/)** — try the real
interface in your browser

![tuki running in a terminal, with tasks grouped under WORK, HOME, HOBBY and
MISC, completed ones crossed out, one overdue in red, and a 3/8 done
counter](docs/screenshot.png)

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/rmpato/tuki/main/install.sh | sh
```

Checks the download against the release's sha256 and installs to
`~/.local/bin`. Also `go install github.com/rmpato/tuki@latest`, or a binary
from [releases](https://github.com/rmpato/tuki/releases).

## Start

```sh
tuki add "buy oat milk #home @tomorrow"   # remember something
tuki                                      # open the interface
```

Completed tasks stay where they are, crossed out and faded, until you clear
them. Finishing one gets a brief, quiet reaction and nothing more.

| key | |
|---|---|
| `↑` `↓` | move — `g` `G` for the ends |
| `space` | complete |
| `a` `e` `d` `u` | add · edit · delete · undo |
| `t` `D` | change group · set due date |
| `tab` `/` `c` | filter · search · clear done |
| `?` `q` | every key · quit |

It works in pipes too:

```sh
tuki list --todo --tag work
tuki done 12
tuki list --json | jq '.[] | select(.tag == "work")'
tuki path                    # where the file lives
```

Groups and due dates can be flags or inline. Dates read the way you'd say
them: `today`, `fri`, `+3d`, `2w`, `2026-08-20`.

## Your tasks

```
~/.local/share/tuki/tasks.json
```

One file, written atomically, readable in any editor. `TUKI_FILE` moves it
elsewhere. The only settings are `celebrate` and `judge`, both in that file —
turn them off and tuki goes quiet.

## mori

[mori](https://github.com/rmpato/mori) is tuki's sibling — a journal, one
Markdown file per day. Same design language, opposite direction: tuki's eyes
are open and pointed forwards, mori's are closed and pointed back.

> tuki helps you move forward. mori helps you look back.

## More

[runbooks](runbooks/) ·
[releases](https://github.com/rmpato/tuki/releases) ·
[MIT](LICENSE)

## Contributing

Bug reports and ideas are welcome — [CONTRIBUTING.md](CONTRIBUTING.md) says how
things are built and which ideas get turned down, so a no is never a surprise.
Vulnerabilities go through [private reporting](https://github.com/rmpato/tuki/security/advisories/new),
not public issues: [SECURITY.md](SECURITY.md).
