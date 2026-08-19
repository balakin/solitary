#!/bin/sh
# Installs solitary from its GitHub releases.
#
#   curl -fsSL https://solitary.balakin.io/install.sh | sh
#
# The archives hold one static binary, so this script only has to pick the right
# one, check it against the published checksum and put it somewhere on PATH.
#
#   SOLITARY_VERSION      a tag to install instead of the latest release
#   SOLITARY_INSTALL_DIR  where to put the binary, instead of the search below
set -eu

REPO="balakin/solitary"
RELEASES="https://github.com/${REPO}/releases"

die() {
	echo "install.sh: $*" >&2
	exit 1
}

note() {
	echo "==> $*"
}

# curl or wget, whichever the machine has: this script is usually piped from
# one of the two, but not always the same one.
if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL -o "$1" "$2"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -qO "$1" "$2"; }
else
	die "neither curl nor wget found"
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
darwin | linux) ;;
*) die "unsupported operating system: $(uname -s); solitary runs on macOS and Linux" ;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch="amd64" ;;
arm64 | aarch64) arch="arm64" ;;
*) die "unsupported architecture: $arch; releases cover amd64 and arm64" ;;
esac

# Homebrew keeps its own copy and would overwrite this one on the next upgrade,
# so let it stay the one thing that manages the install.
if existing=$(command -v solitary 2>/dev/null) && command -v brew >/dev/null 2>&1; then
	prefix=$(brew --prefix 2>/dev/null || true)
	case "${prefix:+$existing}" in
	"$prefix"/*) die "solitary is installed by Homebrew at $existing; upgrade it with: brew upgrade solitary" ;;
	esac
fi

# Where to put it: an explicit choice, then the usual system-wide directory if
# it can be written without sudo, then the per-user one. Nothing here escalates
# privileges — a script read off the network should not be asking for a
# password.
if [ -n "${SOLITARY_INSTALL_DIR:-}" ]; then
	dir=$SOLITARY_INSTALL_DIR
	mkdir -p "$dir" || die "cannot create $dir"
elif [ -w /usr/local/bin ]; then
	dir="/usr/local/bin"
else
	dir="$HOME/.local/bin"
	mkdir -p "$dir" || die "cannot create $dir"
fi

[ -w "$dir" ] || die "$dir is not writable; set SOLITARY_INSTALL_DIR to somewhere that is"

archive="solitary_${os}_${arch}.tar.gz"
if [ -n "${SOLITARY_VERSION:-}" ]; then
	base="${RELEASES}/download/${SOLITARY_VERSION}"
else
	base="${RELEASES}/latest/download"
fi

tmp=$(mktemp -d)
# shellcheck disable=SC2064 # $tmp is expanded now on purpose: it cannot change.
trap "rm -rf '$tmp'" EXIT INT TERM

note "Downloading ${archive}"
fetch "$tmp/$archive" "$base/$archive" ||
	die "no ${archive} at ${base}; check the tag at ${RELEASES}"
fetch "$tmp/checksums.txt" "$base/checksums.txt" ||
	die "cannot download checksums.txt from ${base}"

# An unverified download is the whole risk of installing this way, so a machine
# without a checksum tool is told rather than quietly trusted.
if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
	die "neither sha256sum nor shasum found; cannot verify the download"
fi

want=$(grep " ${archive}\$" "$tmp/checksums.txt" | cut -d' ' -f1)
[ -n "$want" ] || die "checksums.txt names no ${archive}"
got=$(sha256 "$tmp/$archive")
[ "$want" = "$got" ] || die "checksum mismatch for ${archive}: expected ${want}, got ${got}"

tar -xzf "$tmp/$archive" -C "$tmp" solitary || die "cannot extract ${archive}"

# Written inside the target directory rather than moved across filesystems, so
# the rename is atomic and replaces a running binary without breaking it.
staged="$dir/.solitary.$$"
cp "$tmp/solitary" "$staged"
chmod 755 "$staged"
mv -f "$staged" "$dir/solitary"

note "Installed $("$dir/solitary" --version) to $dir/solitary"

case ":$PATH:" in
*":$dir:"*) ;;
*) note "$dir is not on PATH; add it with: export PATH=\"$dir:\$PATH\"" ;;
esac

# solitary drives limactl on the host and has nothing to run without it.
command -v limactl >/dev/null 2>&1 || cat <<'EOF'
==> Lima is not installed. solitary runs cells in Lima machines and needs it:
      macOS   brew install lima
      Linux   see https://lima-vm.io/docs/installation/
EOF
