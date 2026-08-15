#!/bin/sh
# artifact — hand a file out of this cell, or see what is waiting to come in.
#
# Installed by solitary. A cell has no other way out: nothing is mounted from
# the host, so the outbox is where you leave something for whoever is running
# this cell to collect with 'solitary fetch'.
#
# Publishing does not send anything anywhere. It puts a copy in the outbox; the
# host decides when, and whether, to take it.
set -eu

outbox="$HOME/outbox"
inbox="$HOME/inbox"

usage() {
	cat <<'USAGE'
Usage:
  artifact <file>...   publish these, for the host to fetch
  artifact --list      what is published, and what is waiting in the inbox

Published files are collected on the host with:  solitary fetch <cell>
Files sent to this cell arrive in:               $HOME/inbox
USAGE
}

list() {
	echo "outbox — published, waiting to be fetched:"
	if [ -n "$(find "$outbox" -maxdepth 1 -type f -print -quit 2>/dev/null)" ]; then
		ls -lh "$outbox" | tail -n +2 | awk '{printf "  %-10s %s\n", $5, $9}'
	else
		echo "  (nothing)"
	fi

	echo
	echo "inbox — sent to this cell:"
	if [ -n "$(find "$inbox" -maxdepth 1 -type f -print -quit 2>/dev/null)" ]; then
		ls -lh "$inbox" | tail -n +2 | awk '{printf "  %-10s %s\n", $5, $9}'
	else
		echo "  (nothing)"
	fi
}

case "${1:-}" in
"") usage; exit 2 ;;
--list | -l | ls) list; exit 0 ;;
--help | -h) usage; exit 0 ;;
esac

mkdir -p "$outbox"
status=0
for path in "$@"; do
	if [ ! -f "$path" ]; then
		echo >&2 "artifact: $path is not a file"
		status=1
		continue
	fi

	# The name is what the host sees, and it will refuse one it cannot treat
	# as a plain file name — so say so here, where the file still is.
	name=$(basename -- "$path")
	case "$name" in
	.. | . | -*)
		echo >&2 "artifact: $name cannot be published under that name"
		status=1
		continue
		;;
	esac

	cp -- "$path" "$outbox/$name"
	echo "published $name"
done

exit $status
