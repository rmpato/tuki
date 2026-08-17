# Regenerate the screenshot

`docs/screenshot.png` is a real capture of tuki in a Ghostty window. It's used
by both the README and the site. This is fiddlier than it looks, so the
gotchas are worth reading first.

## Rules

- **Never use your own tasks.** Point tuki at a throwaway file with
  `TUKI_HOME`. Your real list stays untouched and out of the picture.
- **Set the terminal background to tuki's page background** — `#14120f`, with
  `#e8e3da` in front of it. Ghostty's default is nearly black, which sits on
  the site as a colder, darker rectangle instead of belonging to it. mori's
  screenshot does the same with its own `#111410`.
- **Never `pkill` Ghostty to avoid the tab problem.** If you're running inside
  Ghostty — and if you're reading this in a terminal, you probably are — that
  takes your shell and anything running in it with you. Open a second instance
  and close it by PID. See the gotchas.
- **84×27 cells.** Wide enough for tuki's 76-column layout with a margin,
  27 rows so all four groups fit without scrolling. Recount if you change the
  demo tasks: header 4 + footer 2 + 5 lines per group + 1 per task.

## Set up the demo list

```sh
WORK=$(mktemp -d)
go build -o "$WORK/tuki" .
export TUKI_HOME="$WORK/data"
T="$WORK/tuki"

$T add "ship the release notes" --tag work
$T add "write the docs"         --tag work  --due fri
$T add "reply to sam"           --tag work  --due 2026-08-14   # deliberately overdue
$T add "buy oat milk"           --tag home  --due tomorrow
$T add "water the plants"       --tag home
$T add "learn one more chord"   --tag hobby
$T add "finish the zine"        --tag hobby
$T add "renew the passport"                 --due 2026-09-12
$T done 1 5 7
```

That set is chosen to show every visual state at once: all four groups, done
and pending, a due date, an overdue one in red, and a partial progress bar.
Keep the overdue date in the past relative to when you shoot.

## Take it

Ghostty needs a launcher script and a config file — see the gotchas for why
neither `-e` nor CLI flags alone will do:

```sh
cat > "$WORK/run.sh" <<EOF
#!/bin/zsh
export TUKI_HOME="$WORK/data"
exec "$WORK/tuki"
EOF
chmod +x "$WORK/run.sh"

cat > "$WORK/ghostty.conf" <<EOF
command = $WORK/run.sh
window-save-state = never
window-width = 84
window-height = 27
window-position-x = 120
window-position-y = 120
font-size = 20
window-padding-x = 18
window-padding-y = 16
background = #14120f
foreground = #e8e3da
title = tuki
EOF

BEFORE=$(ps -Ao pid,comm | grep 'MacOS/ghostty' | awk '{print $1}' | tr '\n' '|' | sed 's/|$//')
open -na Ghostty --args --config-file="$WORK/ghostty.conf"
sleep 6
PID=$(ps -Ao pid,comm | grep 'MacOS/ghostty' | awk '{print $1}' | grep -vE "^(${BEFORE})$" | head -1)
```

Ask macOS where that window is, rather than measuring it by hand, and capture
exactly that rectangle — nothing else on your screen ends up in the file:

```sh
osascript -e "tell application \"System Events\" to tell (first process whose unix id is $PID) \
  to get {position, size} of front window"
# 120, 158, 1044, 762

screencapture -x -o -R 120,158,1044,762 "$WORK/win.png"
```

Then scale to 1600px wide and round the corners:

```sh
sips -Z 1600 "$WORK/win.png" --out "$WORK/scaled.png"
sips -s format bmp "$WORK/scaled.png" --out "$WORK/scaled.bmp"
python3 tools/round_corners.py "$WORK/scaled.bmp" docs/screenshot.png 14
```

Finally, close **only your window**:

```sh
kill "$PID"
rm -rf "$WORK"
```

## Gotchas

**Ghostty remembers its window size.** A new `font-size` with a remembered
pixel size just means fewer visible cells, and the bottom of the list gets
cut. `window-save-state = never` is what prevents it.

**Use a config file, not `-e`.** Launching `open -na Ghostty --args … -e
script.sh` raises a macOS "Allow Ghostty to execute…" dialog, which lands on
top of the window you're trying to photograph. `command =` in a config file
doesn't.

**Don't quit Ghostty to avoid the tab problem.** `open -na` opens a separate
instance, which is enough; close that one by PID. Quitting takes your own
session with it.

**Capture the window, not the screen.** `screencapture` with no `-R` takes the
whole display, which means a file containing your desktop that you then have
to remember to delete. Asking System Events for the window rect means that
file never exists — and it works even when the window opens on another
display, where the coordinates come back negative and `-R` handles them
anyway.

**Round the corners.** The window's own rounded corners let a sliver of
whatever was behind show through at each corner. `tools/round_corners.py`
masks them to transparent; without it you get four bright specks, and GitHub
can't fix it with CSS the way the site can.

## Check

- 1600px wide, a couple of hundred KB at most.
- The background matches the site's `--bg`. Sample a pixel if unsure; it
  should read `#14120f`, not near-black.
- All four group headings present, and the footer key hints visible.
- Corners transparent, no desktop showing.
- Nothing of yours in the frame.
