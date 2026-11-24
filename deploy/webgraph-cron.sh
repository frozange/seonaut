#!/bin/bash
# Quarterly Common Crawl domain graph for the audit's popularity section.
# Run by root from /etc/cron.d/seonaut-webgraph at minute 41, under flock.
# Sestaro reads Common Crawl from minute 07 (2 to 35 min) and at 01:41, from
# the same IP: we download only from minute 41 to 04, never in hour 01, never
# while a Sestaro Common Crawl command runs. One connection, capped rate,
# resumed by byte range; an error status pauses until the next slot. Root
# only looks at Sestaro's processes (hidepid); lafactor writes. CHANGELOG 1.8.0.
set -uo pipefail

BIN=/usr/local/bin/seonaut-webgraph
DIR=/srv/seonaut-webgraph
LOG=$DIR/update.log
RATE=${WEBGRAPH_RATE:-6}   # MB/s
U=lafactor
APP=/home/$U/seonaut

[ -x "$BIN" ] || exit 0
[ "$(stat -c%s "$LOG" 2>/dev/null || echo 0)" -gt 5242880 ] && sudo -u "$U" -H sh -c ": > '$LOG'"
log() { printf '%s %s\n' "$(date '+%F %T')" "$*" | sudo -u "$U" -H tee -a "$LOG" >/dev/null; }

sestaro_busy() { pgrep -f 'artisan sestaro:(enrich-commoncrawl|commoncrawl-index-mirror)' >/dev/null; }

# Seconds left in the free window (minute 41 to minute 04), 0 outside it.
window_left() {
	local h m s
	h=$((10#$(date +%H))) m=$((10#$(date +%M))) s=$((10#$(date +%S)))
	[ "$h" -eq 1 ] && { echo 0; return; }
	if [ "$m" -ge 41 ]; then echo $(( (64 - m) * 60 - s ));
	elif [ "$m" -lt 4 ]; then echo $(( (4 - m) * 60 - s ));
	else echo 0; fi
}

while :; do
	left=$(window_left)
	[ "$left" -ge 30 ] || break
	if sestaro_busy; then
		log "sestaro reads Common Crawl, waiting"
		sleep 60
		continue
	fi
	slot=$(( left < 120 ? left : 120 ))
	out=$(sudo -u "$U" -H nice -n 15 ionice -c2 -n7 "$BIN" -dir "$DIR" -rate "$RATE" update -for "${slot}s" 2>&1)
	rc=$?
	printf '%s\n' "$out" | sudo -u "$U" -H tee -a "$LOG" >/dev/null
	# A quarter just indexed: reports without popularity, or without the
	# comparison now possible, get it.
	if grep -q 'update: built' <<<"$out"; then
		sudo -u "$U" -H sh -c "cd '$APP' && ./auditctl -c config popularity -missing" 2>&1 | sudo -u "$U" -H tee -a "$LOG" >/dev/null
	fi
	[ "$rc" -eq 10 ] || break
done
exit 0
