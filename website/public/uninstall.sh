#!/bin/sh
# Removes a solitary installed by install.sh.
#
#   curl -fsSL https://solitary.balakin.io/uninstall.sh | sh
#
# The binary is the only thing this removes on its own. Cells outlive it — a
# machine is a multi-gigabyte disk Lima keeps, and the definitions hold the
# secrets a cell was given — so both are opt-in and named below.
#
#   SOLITARY_INSTALL_DIR   where to look for the binary, instead of the search below
#   SOLITARY_REMOVE_CELLS  1 to destroy every machine solitary created
#   SOLITARY_PURGE         1 to delete the cell definitions, secrets and state
set -eu

die() {
	echo "uninstall.sh: $*" >&2
	exit 1
}

note() {
	echo "==> $*"
}

# Where the binary is: an explicit choice, then whatever is on PATH, then the
# two directories install.sh picks between.
if [ -n "${SOLITARY_INSTALL_DIR:-}" ]; then
	binary="$SOLITARY_INSTALL_DIR/solitary"
	[ -e "$binary" ] || die "no solitary at $binary"
elif found=$(command -v solitary 2>/dev/null); then
	binary=$found
elif [ -e /usr/local/bin/solitary ]; then
	binary="/usr/local/bin/solitary"
elif [ -e "$HOME/.local/bin/solitary" ]; then
	binary="$HOME/.local/bin/solitary"
else
	binary=""
fi

# Homebrew keeps its own copy under its prefix, and deleting the file out from
# under it leaves the formula installed. Say so and leave it alone.
brewed=""
if [ -n "$binary" ] && command -v brew >/dev/null 2>&1; then
	prefix=$(brew --prefix 2>/dev/null || true)
	case "${prefix:+$binary}" in
	"$prefix"/*) brewed=1 ;;
	esac
fi

# Cells first, while solitary is still installed: once the binary is gone the
# machines are still there, and the tool that knows how to remove them is not.
machines=""
if command -v limactl >/dev/null 2>&1; then
	machines=$(limactl list --quiet 2>/dev/null | grep '^solitary-' || true)
fi

if [ -n "$machines" ] && [ "${SOLITARY_REMOVE_CELLS:-}" != "1" ]; then
	echo "These cells still have a machine on this host:" >&2
	echo "$machines" | sed 's/^solitary-/  /' >&2
	cat >&2 <<'EOF'

Remove them first — 'solitary rm <name>' for each, or 'solitary ls' to see
them — or re-run with SOLITARY_REMOVE_CELLS=1 to destroy every machine above
and its disk:

  curl -fsSL https://solitary.balakin.io/uninstall.sh | SOLITARY_REMOVE_CELLS=1 sh
EOF
	exit 1
fi

if [ -n "$machines" ]; then
	echo "$machines" | while IFS= read -r machine; do
		note "Destroying ${machine#solitary-}"
		limactl delete --force "$machine" >/dev/null || die "cannot delete $machine"
	done
fi

if [ -z "$binary" ]; then
	note "No solitary binary found; set SOLITARY_INSTALL_DIR if it is somewhere unusual"
elif [ -n "$brewed" ]; then
	note "Homebrew installed $binary; remove it with: brew uninstall solitary"
else
	rm -f "$binary" || die "cannot remove $binary"
	note "Removed $binary"
fi

# The config tree is os.UserConfigDir, which is ~/Library/Application Support on
# macOS and ~/.config elsewhere; both are listed because a cell created under
# one is not found by looking in the other.
dirs=""
for dir in \
	"${XDG_CONFIG_HOME:-$HOME/.config}/solitary" \
	"$HOME/Library/Application Support/solitary" \
	"${XDG_STATE_HOME:-$HOME/.local/state}/solitary"; do
	[ -d "$dir" ] || continue
	dirs="${dirs}${dir}
"
done

if [ -n "$dirs" ] && [ "${SOLITARY_PURGE:-}" = "1" ]; then
	echo "$dirs" | while IFS= read -r dir; do
		[ -n "$dir" ] || continue
		rm -rf "$dir" || die "cannot remove $dir"
		note "Removed $dir"
	done
elif [ -n "$dirs" ]; then
	note "Cell definitions and secrets are kept:"
	echo "$dirs" | sed '/^$/d;s/^/      /'
	note "Delete them with SOLITARY_PURGE=1, or by hand"
fi

# Lima is installed separately and may well be running something else.
if command -v limactl >/dev/null 2>&1; then
	note "Lima is left installed; remove it separately if nothing else uses it"
fi

exit 0
