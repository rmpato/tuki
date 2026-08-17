#!/bin/sh
#
# Install tuki.
#
#   curl -fsSL https://raw.githubusercontent.com/rmpato/tuki/main/install.sh | sh
#
# Options (pass with `| sh -s -- <options>`):
#
#   --dir <path>        where to install        (default: ~/.local/bin)
#   --version <tag>     which release to get    (default: the latest)
#   --no-modify-path    don't touch your shell config
#
# The download is checked against the release's sha256 checksum before it is
# installed. Nothing runs as root and nothing is written outside the install
# directory and, unless you opt out, one line in your shell config.

set -eu

REPO="rmpato/tuki"
BIN="tuki"

INSTALL_DIR="${TUKI_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${TUKI_VERSION:-}"
MODIFY_PATH=1

# ---------------------------------------------------------------- output ---

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	C_DIM="$(printf '\033[2m')"
	C_ACCENT="$(printf '\033[38;5;179m')"
	C_ERR="$(printf '\033[31m')"
	C_OFF="$(printf '\033[0m')"
else
	C_DIM='' C_ACCENT='' C_ERR='' C_OFF=''
fi

say() { printf '  %s\n' "$*"; }
dim() { printf '  %s%s%s\n' "$C_DIM" "$*" "$C_OFF"; }
die() {
	printf '\n  %s(x_x)%s  %s\n\n' "$C_ERR" "$C_OFF" "$*" >&2
	exit 1
}

# ------------------------------------------------------------ arguments ---

while [ $# -gt 0 ]; do
	case "$1" in
	--dir)
		[ $# -ge 2 ] || die "--dir needs a path"
		INSTALL_DIR="$2"
		shift 2
		;;
	--version)
		[ $# -ge 2 ] || die "--version needs a tag, like v0.2.0"
		VERSION="$2"
		shift 2
		;;
	--no-modify-path)
		MODIFY_PATH=0
		shift
		;;
	-h | --help)
		# Printed inline rather than read back from $0, because piping this
		# script into sh means there is no file to read.
		cat <<'USAGE'
Install tuki.

  curl -fsSL https://raw.githubusercontent.com/rmpato/tuki/main/install.sh | sh

Options (pass with `| sh -s -- <options>`):

  --dir <path>        where to install        (default: ~/.local/bin)
  --version <tag>     which release to get    (default: the latest)
  --no-modify-path    don't touch your shell config
USAGE
		exit 0
		;;
	*) die "unknown option: $1" ;;
	esac
done

# -------------------------------------------------------------- helpers ---

need() { command -v "$1" >/dev/null 2>&1; }

download() { # download <url> <destination>
	if need curl; then
		curl -fsSL "$1" -o "$2"
	elif need wget; then
		wget -qO "$2" "$1"
	else
		die "tuki needs curl or wget to download anything"
	fi
}

fetch() { # fetch <url>, to stdout
	if need curl; then
		curl -fsSL "$1"
	elif need wget; then
		wget -qO- "$1"
	else
		die "tuki needs curl or wget to download anything"
	fi
}

sha256() { # sha256 <file>
	if need sha256sum; then
		sha256sum "$1" | cut -d' ' -f1
	elif need shasum; then
		shasum -a 256 "$1" | cut -d' ' -f1
	elif need openssl; then
		openssl dgst -sha256 "$1" | sed 's/.*= *//'
	else
		die "tuki needs sha256sum, shasum, or openssl to verify the download"
	fi
}

# ------------------------------------------------------------- platform ---

# detect_platform sets OS and ARCH. It assigns globals rather than printing,
# because `die` inside a $(...) would only kill the subshell and let the rest
# of the script carry on with nothing set.
OS=""
ARCH=""
detect_platform() {
	case "$(uname -s)" in
	Darwin) OS="darwin" ;;
	Linux) OS="linux" ;;
	MINGW* | MSYS* | CYGWIN*) OS="windows" ;;
	*) die "tuki has no build for $(uname -s)" ;;
	esac

	case "$(uname -m)" in
	x86_64 | amd64) ARCH="amd64" ;;
	arm64 | aarch64) ARCH="arm64" ;;
	*) die "tuki has no build for $(uname -m)" ;;
	esac
}

latest_version() {
	# Pull "tag_name": "v0.2.0" out of the API response without needing jq.
	fetch "https://api.github.com/repos/$REPO/releases/latest" |
		tr ',' '\n' |
		sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
		head -n 1
}

# --------------------------------------------------------- shell config ---

# rc_file prints the shell config that should carry the PATH line.
rc_file() {
	shell_name="$(basename "${SHELL:-sh}")"
	case "$shell_name" in
	zsh) printf '%s' "${ZDOTDIR:-$HOME}/.zshrc" ;;
	fish) printf '%s' "$HOME/.config/fish/config.fish" ;;
	bash)
		if [ -f "$HOME/.bashrc" ]; then
			printf '%s' "$HOME/.bashrc"
		elif [ -f "$HOME/.bash_profile" ]; then
			printf '%s' "$HOME/.bash_profile"
		else
			printf '%s' "$HOME/.profile"
		fi
		;;
	*) printf '%s' "$HOME/.profile" ;;
	esac
}

on_path() { # on_path <dir>
	case ":${PATH}:" in
	*":$1:"*) return 0 ;;
	*) return 1 ;;
	esac
}

add_to_path() { # add_to_path <dir>
	dir="$1"
	rc="$(rc_file)"
	shell_name="$(basename "${SHELL:-sh}")"

	if [ "$shell_name" = "fish" ]; then
		line="fish_add_path $dir"
	else
		line="export PATH=\"$dir:\$PATH\""
	fi

	# Don't add a second copy if the line is already there. Existing configs
	# often write the path as "$HOME/.local/bin", so check that form too.
	dir_home="$(printf '%s' "$dir" | sed "s|^$HOME|\$HOME|")"
	if [ -f "$rc" ] && { grep -Fq "$dir" "$rc" || grep -Fq "$dir_home" "$rc"; }; then
		dim "$rc already mentions $dir"
		PATH_NOTE="$rc"
		return 0
	fi

	mkdir -p "$(dirname "$rc")"
	{
		printf '\n# added by tuki install.sh\n'
		printf '%s\n' "$line"
	} >>"$rc" || {
		PATH_NOTE=""
		dim "couldn't write to $rc — add this line yourself:"
		dim "  $line"
		return 0
	}

	say "added $dir to your PATH in $C_ACCENT$rc$C_OFF"
	PATH_NOTE="$rc"
}

# --------------------------------------------------------------- install ---

printf '\n  %s(o.o)%s  %stuki%s\n\n' "$C_ACCENT" "$C_OFF" "$C_ACCENT" "$C_OFF"

detect_platform

if [ -z "$VERSION" ]; then
	dim "looking up the latest release..."
	VERSION="$(latest_version)"
	[ -n "$VERSION" ] || die "couldn't work out the latest version.
  Pass one with --version <tag>, from https://github.com/$REPO/releases"
fi

# The archive name has to match .goreleaser.yaml's name_template.
NUMERIC="${VERSION#v}"
EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"
ARCHIVE="${BIN}_${NUMERIC}_${OS}_${ARCH}.${EXT}"
BASE="https://github.com/$REPO/releases/download/$VERSION"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

say "downloading $C_ACCENT$BIN $VERSION$C_OFF for $OS/$ARCH"
download "$BASE/$ARCHIVE" "$TMP/$ARCHIVE" ||
	die "couldn't download $ARCHIVE — does $VERSION have a build for $OS/$ARCH?"
download "$BASE/checksums.txt" "$TMP/checksums.txt" ||
	die "couldn't download checksums.txt, so the download can't be verified"

# Compare filenames literally rather than as a regex — the archive name is full
# of dots, which would otherwise match more than intended.
EXPECTED="$(awk -v want="$ARCHIVE" '{ name = $2; sub(/^\*/, "", name); if (name == want) { print $1; exit } }' "$TMP/checksums.txt")"
[ -n "$EXPECTED" ] || die "$ARCHIVE isn't listed in checksums.txt"

ACTUAL="$(sha256 "$TMP/$ARCHIVE")"
if [ "$ACTUAL" != "$EXPECTED" ]; then
	die "checksum mismatch for $ARCHIVE
    expected $EXPECTED
    got      $ACTUAL
  not installing it."
fi
dim "checksum ok"

if [ "$EXT" = "zip" ]; then
	need unzip || die "tuki needs unzip to unpack the download"
	unzip -q "$TMP/$ARCHIVE" -d "$TMP"
else
	tar -xzf "$TMP/$ARCHIVE" -C "$TMP"
fi
[ -f "$TMP/$BIN" ] || die "the archive didn't contain $BIN"

mkdir -p "$INSTALL_DIR" || die "couldn't create $INSTALL_DIR"
install -m 0755 "$TMP/$BIN" "$INSTALL_DIR/$BIN" 2>/dev/null ||
	{ cp "$TMP/$BIN" "$INSTALL_DIR/$BIN" && chmod 0755 "$INSTALL_DIR/$BIN"; } ||
	die "couldn't write to $INSTALL_DIR — try --dir ~/.local/bin"

say "installed to $C_ACCENT$INSTALL_DIR/$BIN$C_OFF"

# ------------------------------------------------------------------ path ---

PATH_NOTE=""
if on_path "$INSTALL_DIR"; then
	ALREADY_ON_PATH=1
else
	ALREADY_ON_PATH=0
	if [ "$MODIFY_PATH" -eq 1 ]; then
		add_to_path "$INSTALL_DIR"
	else
		dim "$INSTALL_DIR isn't on your PATH (--no-modify-path was given)"
	fi
fi

printf '\n'
if [ "$ALREADY_ON_PATH" -eq 1 ]; then
	say "run $C_ACCENT$BIN$C_OFF to get started."
elif [ -n "$PATH_NOTE" ]; then
	say "open a new terminal, or run:"
	printf '\n      %ssource %s%s\n\n' "$C_ACCENT" "$PATH_NOTE" "$C_OFF"
	say "then run $C_ACCENT$BIN$C_OFF to get started."
else
	say "run it with $C_ACCENT$INSTALL_DIR/$BIN$C_OFF"
fi
printf '\n'
