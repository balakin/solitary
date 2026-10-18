#!/bin/sh
# Installs solitary from its GitHub releases.
#
#   curl -fsSL https://solitary.balakin.io/install.sh | sh
#
# The archives hold one static binary, so this script only has to pick the right
# one, authenticate its checksum manifest and put it somewhere on PATH.
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
	fetch() { curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL -o "$1" "$2"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget --https-only -qO "$1" "$2"; }
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

if [ -n "${SOLITARY_VERSION:-}" ]; then
	base="${RELEASES}/download/${SOLITARY_VERSION}"
else
	base="${RELEASES}/latest/download"
fi

tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT INT TERM

if command -v sha256sum >/dev/null 2>&1; then
 sha256() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
 sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
 die "neither sha256sum nor shasum found; cannot verify the download"
fi

# Bootstrap only a verifier whose digest is pinned in this installer.
if command -v cosign >/dev/null 2>&1; then
 cosign_bin=cosign
else
 case "${os}_${arch}" in
  linux_amd64) verifier_sha=c956e5dfcac53d52bcf058360d579472f0c1d2d9b69f55209e256fe7783f4c74 ;;
  linux_arm64) verifier_sha=bedac92e8c3729864e13d4a17048007cfafa79d5deca993a43a90ffe018ef2b8 ;;
  darwin_amd64) verifier_sha=4c3e7af8372d3ca3296e62fa56f23fcbb5721cc6ac1827900d398f110d7cd280 ;;
  darwin_arm64) verifier_sha=5fadd012ae6381a6a29ff86a7d39aa873878852f1073fc90b15995961ecfb084 ;;
 esac
 cosign_bin="$tmp/cosign"
 note "Downloading pinned signature verifier"
 fetch "$cosign_bin" "https://github.com/sigstore/cosign/releases/download/v3.0.6/cosign-${os}-${arch}" || die "cannot download signature verifier"
 [ "$(sha256 "$cosign_bin")" = "$verifier_sha" ] || die "signature verifier checksum mismatch"
 chmod 700 "$cosign_bin"
fi
fetch "$tmp/checksums.txt" "$base/checksums.txt" || die "cannot download release checksums"
fetch "$tmp/checksums.txt.sigstore.json" "$base/checksums.txt.sigstore.json" || die "release has no signature bundle; unsigned releases are refused"
"$cosign_bin" verify-blob \
 --certificate-identity 'https://github.com/balakin/solitary/.github/workflows/release.yml@refs/heads/main' \
 --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
 --bundle "$tmp/checksums.txt.sigstore.json" "$tmp/checksums.txt" || die "release signature verification failed"

if [ -n "${SOLITARY_VERSION:-}" ]; then
 version=${SOLITARY_VERSION#v}
 case "$version" in *[!0-9A-Za-z.+-]*|'') die "invalid release version" ;; esac
 archive="solitary_${version}_${os}_${arch}.tar.gz"
else
 archive=$(awk -v suffix="_${os}_${arch}.tar.gz" '$2 ~ /^solitary_[0-9][0-9A-Za-z.+_-]*$/ && substr($2,length($2)-length(suffix)+1)==suffix {print $2}' "$tmp/checksums.txt")
 case "$archive" in ''|*'
'*) die "release checksums must name exactly one matching archive" ;; esac
fi
note "Downloading ${archive}"
fetch "$tmp/$archive" "$base/$archive" || die "cannot download ${archive}"

want=$(awk -v name="$archive" '$2 == name {print $1}' "$tmp/checksums.txt")
[ -n "$want" ] || die "checksums.txt names no ${archive}"
got=$(sha256 "$tmp/$archive")
[ "$want" = "$got" ] || die "checksum mismatch for ${archive}: expected ${want}, got ${got}"

tar -xzf "$tmp/$archive" -C "$tmp" solitary || die "cannot extract ${archive}"

# Written inside the target directory rather than moved across filesystems, so
# the rename is atomic and replaces a running binary without breaking it.
staged=$(mktemp "$dir/.solitary.XXXXXXXX")
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
