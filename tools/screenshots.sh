#!/bin/zsh
#
# Capture every screenshot the site uses, into docs/.
#
#   ./tools/screenshots.sh
#
# Opens a second Ghostty instance against a throwaway task file, drives it with
# keystrokes, captures each screen, and closes only the window it opened. Your
# own tasks and your own terminal are never touched — see
# runbooks/screenshot.md for why that second part matters.
#
# Needs: Ghostty, and Accessibility permission for whatever runs this (System
# Events is used to find the window and to type into it).

set -eu

ROOT="${0:A:h:h}"
OUT="$ROOT/docs"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT INT TERM

# tuki's own palette, so the capture blends into the page rather than sitting
# on it as a colder, darker rectangle.
BG="#14120f"
FG="#e8e3da"

# ------------------------------------------------------------------ setup --

go build -o "$WORK/tuki" "$ROOT"

# A list chosen to show every visual state at once: all four groups, done and
# pending, a due date, an overdue one in red, and a partial progress bar. Keep
# the overdue date in the past relative to when you shoot.
seed() {
  local home="$1" T="$WORK/tuki"
  TUKI_HOME="$home" $T add "ship the release notes" --tag work >/dev/null
  TUKI_HOME="$home" $T add "write the docs"         --tag work  --due fri >/dev/null
  TUKI_HOME="$home" $T add "reply to sam"           --tag work  --due 2026-08-14 >/dev/null
  TUKI_HOME="$home" $T add "buy oat milk"           --tag home  --due tomorrow >/dev/null
  TUKI_HOME="$home" $T add "water the plants"       --tag home >/dev/null
  TUKI_HOME="$home" $T add "learn one more chord"   --tag hobby >/dev/null
  TUKI_HOME="$home" $T add "finish the zine"        --tag hobby >/dev/null
  TUKI_HOME="$home" $T add "renew the passport"                 --due 2026-09-12 >/dev/null
}

seed "$WORK/data"
TUKI_HOME="$WORK/data" "$WORK/tuki" done 1 5 7 >/dev/null

# A second list with nothing left on it, for the state tuki is happiest in.
seed "$WORK/clear"
TUKI_HOME="$WORK/clear" "$WORK/tuki" done 1 2 3 4 5 6 7 8 >/dev/null

# ---------------------------------------------------------------- capture --

# ghostty <rows> <home> <command...> — opens a window and prints its pid.
ghostty() {
  local rows="$1" home="$2"; shift 2
  cat > "$WORK/run.sh" <<EOF
#!/bin/zsh
export TUKI_HOME="$home"
export TUKI_THEME=dark
$@
EOF
  chmod +x "$WORK/run.sh"

  cat > "$WORK/ghostty.conf" <<EOF
command = $WORK/run.sh
window-save-state = never
window-width = 84
window-height = $rows
window-position-x = 120
window-position-y = 120
font-size = 20
window-padding-x = 18
window-padding-y = 16
background = $BG
foreground = $FG
title = tuki
EOF

  local before after
  before=$(ps -Ao pid,comm | grep 'MacOS/ghostty' | awk '{print $1}' | tr '\n' '|' | sed 's/|$//')
  open -na Ghostty --args --config-file="$WORK/ghostty.conf"
  sleep 5
  after=$(ps -Ao pid,comm | grep 'MacOS/ghostty' | awk '{print $1}' | grep -vE "^(${before})$" | head -1)
  [[ -n "$after" ]] || { echo "no new Ghostty window appeared" >&2; exit 1 }
  print -r -- "$after"
}

# shot <pid> <name> — capture exactly that window into docs/<name>.png.
shot() {
  local pid="$1" name="$2" bounds x y w h
  bounds=$(osascript -e "tell application \"System Events\" to tell (first process whose unix id is $pid) to get {position, size} of front window")
  x=${${(s:,:)bounds}[1]// /}; y=${${(s:,:)bounds}[2]// /}
  w=${${(s:,:)bounds}[3]// /}; h=${${(s:,:)bounds}[4]// /}

  screencapture -x -o -R "$x,$y,$w,$h" "$WORK/$name.png"
  sips -Z 1600 "$WORK/$name.png" --out "$WORK/$name-scaled.png" >/dev/null
  sips -s format bmp "$WORK/$name-scaled.png" --out "$WORK/$name.bmp" >/dev/null
  python3 "$ROOT/tools/round_corners.py" "$WORK/$name.bmp" "$OUT/$name.png" 14
}

# send <pid> <keys...> — send keystrokes to that window.
send() {
  local pid="$1"; shift
  osascript -e "tell application \"System Events\" to tell (first process whose unix id is $pid) to set frontmost to true" >/dev/null
  sleep 1
  for key in "$@"; do
    # Named keys go by key code. `keystroke "tab"` types the letters t, a, b,
    # which is three real keypresses in tuki and the wrong screen in the file.
    case "$key" in
      esc)   osascript -e 'tell application "System Events" to key code 53' ;;
      tab)   osascript -e 'tell application "System Events" to key code 48' ;;
      enter) osascript -e 'tell application "System Events" to key code 36' ;;
      space) osascript -e 'tell application "System Events" to key code 49' ;;
      down)  osascript -e 'tell application "System Events" to key code 125' ;;
      *)     osascript -e "tell application \"System Events\" to keystroke \"$key\"" ;;
    esac
    sleep 0.5
  done
  sleep 1
}

# type <pid> <string> — send a whole string at once.
type_text() {
  local pid="$1" text="$2"
  osascript -e "tell application \"System Events\" to tell (first process whose unix id is $pid) to set frontmost to true" >/dev/null
  sleep 0.6
  osascript -e "tell application \"System Events\" to keystroke \"$text\""
  sleep 1.2
}

PID=$(ghostty 27 "$WORK/data" "exec \"$WORK/tuki\"")
shot "$PID" screenshot                                  # the list
send "$PID" a; type_text "$PID" "email sam #work @fri"; shot "$PID" screen-add
send "$PID" esc /; type_text "$PID" "the";              shot "$PID" screen-search
send "$PID" esc '?';                                    shot "$PID" screen-keys
kill "$PID"; sleep 2

PID=$(ghostty 27 "$WORK/clear" "exec \"$WORK/tuki\"")   # all four groups have to fit
shot "$PID" screen-done                                 # everything crossed off
kill "$PID"; sleep 2

# The command line, which needs its own shorter window.
cat > "$WORK/cli.zsh" <<'EOS'
run() { print -r -- ""; print -rP "  %F{179}\$%f $*"; eval "$@"; }
run 'tuki add "email sam #work @fri"'
run 'tuki list --todo --tag work'
run 'tuki done 2'
print -r -- ""
sleep 3600
EOS
chmod +x "$WORK/cli.zsh"
PID=$(ghostty 20 "$WORK/data" "export PATH=\"$WORK:\$PATH\"; exec /bin/zsh \"$WORK/cli.zsh\"")
shot "$PID" screen-cli
kill "$PID"; sleep 1

ls -lh "$OUT"/*.png | awk '{print $9, $5}'
