# Regenerate the screenshot

`docs/screenshot.png` is a real capture of tuki in a Ghostty window. It's used
by both the README and the site. This is fiddlier than it looks, so the
gotchas are worth reading first.

## Rules

- **Never use your own tasks.** Point tuki at a throwaway file with
  `TUKI_HOME`. Your real list stays untouched and out of the picture.
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
font-size = 22
window-padding-x = 18
window-padding-y = 16
title = tuki
EOF

pkill -9 -f "MacOS/ghostty"; sleep 2
open -na Ghostty --args --config-file="$WORK/ghostty.conf"
sleep 5
screencapture -x -o -D 1 "$WORK/full.png"     # -D picks the display
```

Then crop to the window, scale to 1600px wide, and round the corners:

```sh
sips --cropOffset <Y> <X> -c <H> <W> "$WORK/full.png" --out "$WORK/crop.png"
sips -Z 1600 "$WORK/crop.png" --out docs/screenshot.png
sips -s format bmp docs/screenshot.png --out /tmp/shot.bmp
python3 tools/round_corners.py /tmp/shot.bmp docs/screenshot.png 14
```

Finally, **delete the full-screen capture** — it has your whole desktop in it:

```sh
rm -rf "$WORK" /tmp/shot.bmp
pkill -9 -f "MacOS/ghostty"
```

## Gotchas

**Ghostty remembers its window size.** A new `font-size` with a remembered
pixel size just means fewer visible cells, and the bottom of the list gets
cut. `window-save-state = never` is what prevents it.

**Use a config file, not `-e`.** Launching `open -na Ghostty --args … -e
script.sh` raises a macOS "Allow Ghostty to execute…" dialog, which lands on
top of the window you're trying to photograph. `command =` in a config file
doesn't.

**Quit Ghostty completely first.** Otherwise the new window opens as a *tab*
in the existing one, and the tab bar steals two rows.

**The window may open on another display or Space.** `screencapture` only sees
one display at a time; try `-D 1`, `-D 2`, `-D 3`. On a non-Retina external
monitor the result is half the resolution — prefer the built-in display, or
double `font-size` to compensate.

**Round the corners.** The window's own rounded corners let a sliver of
whatever was behind show through at each corner. `tools/round_corners.py`
masks them to transparent; without it you get four bright specks, and GitHub
can't fix it with CSS the way the site can.

## Check

- 1600px wide, a couple of hundred KB at most.
- All four group headings present, and the footer key hints visible.
- Corners transparent, no desktop showing.
- Nothing of yours in the frame.
