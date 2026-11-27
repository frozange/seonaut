#!/bin/bash
# Installs or upgrades SEOnaut in the lafactor account. Run as root on srv.
# Expects in /root/scratch/seonaut-deploy: release.tgz, db.pass, turnstile.secret (first install only).
# Root only reads the release and the secrets: every write under the account's
# home is made by the account itself (sudo -u lafactor), so root never follows
# a link the account could have planted.
set -euo pipefail
S=/root/scratch/seonaut-deploy
U=lafactor
APP=/home/$U/seonaut
as() { sudo -u "$U" -H "$@"; }

[ -f "$S/release.tgz" ] || { echo "missing $S/release.tgz"; exit 1; }
as mkdir -p -m 700 "$APP" "$APP/archive" "$APP/data"

# Release files: replace code, templates and assets; never touch config or data.
# The account unpacks the archive from stdin into a staging folder of the app.
as rm -rf "$APP/release.new"
as mkdir -m 700 "$APP/release.new"
as tar -xzf - -C "$APP/release.new" --no-same-owner --no-same-permissions < "$S/release.tgz"
for d in migrations translations web; do
  [ -d "$APP/release.new/$d" ] || { echo "release has no $d"; exit 1; }
done
for d in migrations translations web; do
  as rm -rf "$APP/$d.old"
  if [ -e "$APP/$d" ]; then as mv "$APP/$d" "$APP/$d.old"; fi
  as mv "$APP/release.new/$d" "$APP/$d"
  as rm -rf "$APP/$d.old"
done
as install -m 750 "$APP/release.new/seonaut" "$APP/seonaut.new"
as mv -f "$APP/seonaut.new" "$APP/seonaut"
if [ -f "$APP/release.new/auditctl" ]; then
  as install -m 750 "$APP/release.new/auditctl" "$APP/auditctl"
fi
as rm -rf "$APP/release.new"

if ! as test -f "$APP/config"; then
  {
    cat <<EOF
[server]
host = "127.0.0.1"
port = 9100
url = "https://seo.lafactory.com"
noindex = true
trusted_proxies = ["127.0.0.1"]
client_ip_header = "CF-Connecting-IP"
session_auth_key = "$(openssl rand -base64 64 | tr -d '\n')"
session_encryption_key = "$(openssl rand -base64 32 | tr -d '\n')"

[database]
server = "127.0.0.1"
port = 3306
user = "lafactor_seonaut"
password = "$(cat "$S/db.pass")"
database = "lafactor_seonaut"

[crawler]
agent = "Mozilla/5.0 (compatible; LaFactorySEOBot/1.0; +https://seo.lafactory.com/bot)"
crawl_limit = 5000
allowed_ports = [80, 443]
deny_addresses = ["178.63.0.108", "178.63.0.91"]

[UI]
language = "en"
theme = "light"

[turnstile]
site_key = "0x4AAAAAAF6CnSVb2gPWY8OB"
secret_key = "$(cat "$S/turnstile.secret")"

[mail]
sendmail = "/usr/sbin/sendmail"
from = "LaFactory SEO <info@lafactory.com>"

[limits]
signups_per_ip_per_day = 5
signin_attempts_per_ip_per_hour = 20
signin_attempts_per_email_per_hour = 10
verification_mails_per_email_per_day = 3
crawls_per_user_per_day = 10
max_concurrent_crawls = 2
max_projects_per_user = 5
allow_custom_user_agent = false
EOF
  } | as bash -c 'umask 077; cat > "$1"' _ "$APP/config"
  as chmod 600 "$APP/config"
fi

# Relay secret of lafactory.com (lf-audit, POST /audit/relay): added once under
# [audit] of an existing config, from relay.secret next to the release.
if [ -f "$S/relay.secret" ] && ! as grep -q '^relay_secret' "$APP/config"; then
  as cat "$APP/config" \
    | awk -v f="$S/relay.secret" 'BEGIN { getline s < f } { print } /^\[audit\]$/ { print "relay_secret = \"" s "\"" }' \
    | as bash -c 'umask 077; cat > "$1.new" && mv -f "$1.new" "$1"' _ "$APP/config"
  as grep -q '^relay_secret' "$APP/config" || { echo "relay_secret not added (no [audit] section?)"; exit 1; }
fi

as bash -c 'umask 077; cat > "$1"' _ "$APP/run.sh" <<'EOF'
#!/bin/bash
# Started every minute by the account's crontab: the lock keeps a single instance alive.
cd "$(dirname "$(readlink -f "$0")")" || exit 1
exec 9>data/.run.lock
flock -n 9 || exit 0
[ "$(stat -c%s data/service.log 2>/dev/null || echo 0)" -gt 10485760 ] && : > data/service.log
exec ./seonaut -c config >> data/service.log 2>&1
EOF
as chmod 700 "$APP/run.sh"

if ! as crontab -l 2>/dev/null | grep -qF "$APP/run.sh"; then
  { as crontab -l 2>/dev/null || true; echo "* * * * * $APP/run.sh"; } | as crontab -
fi

# Common Crawl graph tool (popularity section): root-owned, run as the account
# by /etc/cron.d/seonaut-webgraph. Taken from the archive, never from the account.
W=$S/webgraph-stage
mkdir -p "$W"
if tar -xzf "$S/release.tgz" -C "$W" ./webgraph ./deploy/webgraph-cron.sh 2>/dev/null; then
  install -m 755 -o root -g root "$W/webgraph" /usr/local/bin/seonaut-webgraph
  install -d -m 755 /root/claude/seonaut-webgraph
  install -m 750 "$W/deploy/webgraph-cron.sh" /root/claude/seonaut-webgraph/webgraph-cron.sh
  if [ ! -d /srv/seonaut-exclude ]; then
    install -d -m 750 -o "$U" -g "$U" /srv/seonaut-exclude
  fi
  if [ ! -d /srv/seonaut-webgraph ]; then
    install -d -m 750 -o "$U" -g "$U" /srv/seonaut-webgraph
  fi
  if [ ! -f /etc/cron.d/seonaut-webgraph ]; then
    printf '%s\n' 'SHELL=/bin/bash' 'PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin' 'MAILTO=""' \
      '41 * * * * root /usr/bin/flock -n /run/lock/seonaut-webgraph.lock /root/claude/seonaut-webgraph/webgraph-cron.sh' > /etc/cron.d/seonaut-webgraph
  fi
fi

# Restart: the cron line brings the new binary up within a minute; start it now.
as pkill -u "$U" -f "^./seonaut -c config" || true
sleep 1
as bash -c "nohup $APP/run.sh >/dev/null 2>&1 &"
for _ in $(seq 1 20); do ss -ltn | grep -q "127.0.0.1:9100 " && break; sleep 1; done
ss -ltnp | grep ":9100 " || { echo "NOT LISTENING"; as tail -20 "$APP/data/service.log"; exit 1; }
as tail -3 "$APP/data/service.log"
