#!/bin/bash
# Claude Code statusline for a cell.
#
# Format: [Model] effort | ctx:X% | 5h:X% (2h5m) | 7d:X% (3d4h)
#
# The rate-limit fields say how long is left rather than what the clock will
# read when they reset: a cell has no reason to agree with the host on a
# timezone, and a duration means the same thing in both.

input=$(cat)

model=$(echo "$input" | jq -r '.model.display_name')
effort=$(echo "$input" | jq -r '.effort.level // "--"')

ctx=$(echo "$input" | jq -r '.context_window.used_percentage // empty')
if [ -n "$ctx" ]; then
	ctx=$(printf '%.0f' "$ctx")
else
	ctx="--"
fi

# Two units at most, largest first. The payload carries a unix epoch, so this
# never reads TZ.
remaining() {
	local left=$(($1 - $(date +%s)))
	if ((left <= 0)); then
		printf '0m'
		return
	fi

	local d=$((left / 86400)) h=$((left % 86400 / 3600)) m=$((left % 3600 / 60))
	if ((d > 0)); then
		printf '%dd%dh' "$d" "$h"
	elif ((h > 0)); then
		printf '%dh%dm' "$h" "$m"
	else
		printf '%dm' "$m"
	fi
}

five_pct=$(echo "$input" | jq -r '.rate_limits.five_hour.used_percentage // empty')
five_reset=$(echo "$input" | jq -r '.rate_limits.five_hour.resets_at // empty')
if [ -n "$five_pct" ]; then
	five_pct=$(printf '%.0f' "$five_pct")
else
	five_pct="--"
fi
if [ -n "$five_reset" ]; then
	five_left=$(remaining "$five_reset")
else
	five_left="--"
fi

week_pct=$(echo "$input" | jq -r '.rate_limits.seven_day.used_percentage // empty')
week_reset=$(echo "$input" | jq -r '.rate_limits.seven_day.resets_at // empty')
if [ -n "$week_pct" ]; then
	week_pct=$(printf '%.0f' "$week_pct")
else
	week_pct="--"
fi
if [ -n "$week_reset" ]; then
	week_left=$(remaining "$week_reset")
else
	week_left="--"
fi

printf '[%s] %s | ctx:%s%% | 5h:%s%% (%s) | 7d:%s%% (%s)\n' \
	"$model" "$effort" "$ctx" "$five_pct" "$five_left" "$week_pct" "$week_left"
