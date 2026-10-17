#!/usr/bin/env bash
#
# Puts an uploaded release live. Installed as /usr/local/bin/oriel-activate.
#
# The deploy pipeline reaches this through an SSH key whose only permitted
# command is this script, so the one thing it can ask for is "activate the
# release with this commit id".
#
# The server runs in one of two slots, on ports 8400 and 8401, and nginx sends
# visitors to one of them. A new release is started in the idle slot, beside
# the live one, and must pass its check there: it loads every kind of page,
# books a viewing, requests a reservation, signs in to the sales desk, sees the
# reservation arrive, confirms it and downloads the letter of offer. Only then
# do visitors reach it. The pages and a live-update stream are then checked
# again through nginx; if they fail, nginx goes back to the release before,
# which has been running all along.
#
# To go back by hand, run this script with the commit id of an earlier
# release that is still in the releases directory.

set -euo pipefail

BASE=/var/www/oriel.jeffgicharu.com
HOST=oriel.jeffgicharu.com
APP_USER=oriel
ENV_FILE=$BASE/shared/env
STATIC=$BASE/shared/static
UPSTREAM=/etc/nginx/oriel/upstream.conf
KEEP_RELEASES=3

release_id="${SSH_ORIGINAL_COMMAND:-${1:-}}"

if [[ ! "$release_id" =~ ^[0-9a-f]{40}$ ]]; then
    echo "Expected a 40 character commit id." >&2
    exit 64
fi

release="$BASE/releases/$release_id"

for file in oriel unavailable.html REVISION assets/manifest.json; do
    if [[ ! -f "$release/$file" ]]; then
        echo "Release $release_id has not been uploaded completely ($file is missing)." >&2
        exit 66
    fi
done
if [[ "$(cat "$release/REVISION")" != "$release_id" ]]; then
    echo "Release $release_id carries the wrong REVISION." >&2
    exit 66
fi

# One deploy at a time.
exec 9>"$BASE/.activate.lock"
flock -n 9 || { echo "Another activation is running." >&2; exit 75; }

live_port="$(grep -oE '127\.0\.0\.1:840[01]' "$UPSTREAM" | head -n1 | cut -d: -f2 || true)"
case "$live_port" in
    8400) idle_port=8401 ;;
    8401) idle_port=8400 ;;
    *) echo "Cannot tell which slot is live from $UPSTREAM." >&2; exit 70 ;;
esac
live_release="$(readlink -f "$BASE/slots/$live_port" 2>/dev/null || true)"
first_release=""
if [[ -z "$live_release" || ! -d "$live_release" ]] || ! systemctl is-active --quiet "oriel@$live_port.service"; then
    first_release=yes
fi

# Asked again for the release that is already live: nothing to do.
if [[ -z "$first_release" && "$live_release" == "$release" ]]; then
    echo "Release ${release_id:0:7} is already live on port $live_port."
    exit 0
fi

# nginx is shared by every site on this server. If its configuration is
# already broken by something else, nothing here should touch it.
if ! sudo nginx -t >/dev/null 2>&1; then
    echo "nginx's configuration does not pass its own test before this release changed anything." >&2
    echo "Nothing was changed. Find out which site broke it before deploying again." >&2
    exit 70
fi

chmod 755 "$release/oriel"

# Release code only ever runs as the application user. This script runs as
# deploy, which can become root, so it never executes anything it was sent.
as_app() {
    sudo -u "$APP_USER" -- "$release/oriel" "$@"
}

# The processor here is old. Prove this build runs on it before anything else.
if ! reported="$(as_app version 2>&1)" || [[ "$reported" != "$release_id" ]]; then
    echo "Release ${release_id:0:7} does not run on this server: $reported" >&2
    exit 1
fi

# Fingerprinted files arrive as plain files in the release. A name never
# changes its contents, so files already shared are left as they are.
echo "Sharing this release's scripts, styles, fonts and pictures"
mkdir -p "$STATIC/assets"
rsync -a --ignore-existing --exclude manifest.json "$release/assets/" "$STATIC/assets/"
cp "$release/unavailable.html" "$BASE/shared/pages/unavailable.html"

echo "Starting release ${release_id:0:7} on port $idle_port"
sudo systemctl stop "oriel@$idle_port.service" 2>/dev/null || true
ln -sfn "$release" "$BASE/slots/$idle_port.new"
mv -Tf "$BASE/slots/$idle_port.new" "$BASE/slots/$idle_port"
sudo systemctl start "oriel@$idle_port.service"

abandon() {
    sudo systemctl stop "oriel@$idle_port.service" 2>/dev/null || true
    echo "$1" >&2
    exit 1
}

started=""
for attempt in $(seq 1 40); do
    if curl --silent --max-time 3 --head "http://127.0.0.1:$idle_port/healthz" 2>/dev/null | grep -qi "^x-release: $release_id"; then
        started=yes
        break
    fi
    sleep 1
done
if [[ -z "$started" ]]; then
    sudo journalctl -u "oriel@$idle_port.service" -n 30 --no-pager >&2 || true
    abandon "Release ${release_id:0:7} did not start. The site is unchanged."
fi

# The gate. Visitors are still on the release before, so a release that fails
# here never reaches them. The check reads the settings file itself.
echo "Checking the release before it goes live"
if ! as_app check-release --env-file "$ENV_FILE" --base "http://127.0.0.1:$idle_port" --release "$release_id"; then
    sudo journalctl -u "oriel@$idle_port.service" -n 30 --no-pager >&2 || true
    abandon "Release ${release_id:0:7} failed its check and was not put live."
fi

point_nginx_at() {
    printf 'server 127.0.0.1:%s;\n' "$1" | sudo tee "$UPSTREAM.new" >/dev/null
    sudo mv -f "$UPSTREAM.new" "$UPSTREAM"
    sudo nginx -t >/dev/null 2>&1 || return 1
    sudo systemctl reload nginx
}

# Asks the web server on this machine for a page under the public name. The
# certificate is the one Cloudflare trusts, not one curl knows, hence --insecure.
fetch() {
    curl --silent --show-error --insecure --max-time 20 --resolve "$HOST:443:127.0.0.1" "$@"
}

serves_the_release() {
    local attempt headers paths path stream
    for attempt in 1 2 3 4 5 6 7 8 9 10; do
        headers="$(fetch --head "https://$HOST/healthz" || true)"
        if grep -qi "^x-release: $release_id" <<<"$headers"; then
            break
        fi
        if [[ $attempt -eq 10 ]]; then
            echo "The web server is not serving ${release_id:0:7}." >&2
            return 1
        fi
        sleep 1
    done
    # The release names its own public pages.
    paths="$(as_app check-release --print-paths)" || return 1
    while IFS= read -r path; do
        [[ -z "$path" ]] && continue
        if ! fetch --fail --output /dev/null "https://$HOST$path"; then
            echo "$path did not load on the live site." >&2
            return 1
        fi
    done <<<"$paths"
    # A live-update stream through nginx opens with a retry line. The stream
    # stays open, so curl is cut off after a few seconds and only its output counts.
    stream="$(fetch --max-time 4 "https://$HOST/events" 2>/dev/null || true)"
    if ! grep -q '^retry:' <<<"$stream"; then
        echo "Live updates do not get through nginx." >&2
        return 1
    fi
}

go_back() {
    if [[ -n "$first_release" ]]; then
        abandon "$1 There was no release before this one to go back to."
    fi
    if point_nginx_at "$live_port"; then
        abandon "$1 Visitors are back on the release before."
    fi
    abandon "$1 nginx could not be pointed back. Look at this now."
}

echo "Switching visitors to release ${release_id:0:7}"
if ! point_nginx_at "$idle_port"; then
    # Put things back as they were. nginx was not reloaded.
    printf 'server 127.0.0.1:%s;\n' "$live_port" | sudo tee "$UPSTREAM" >/dev/null
    abandon "nginx refused its configuration. The site is unchanged."
fi

echo "Checking the live site"
if ! serves_the_release; then
    go_back "Release ${release_id:0:7} did not serve its pages."
fi

# The slot that is live is the one that starts when the server boots.
sudo systemctl enable "oriel@$idle_port.service" >/dev/null 2>&1 || true
sudo systemctl disable "oriel@$live_port.service" >/dev/null 2>&1 || true

# Requests in hand on the old release are given time to finish. Stopping it
# ends its live-update streams, and those browsers reconnect to the new release.
sleep 10
sudo systemctl stop "oriel@$live_port.service" 2>/dev/null || true

echo "Removing old releases"
cd "$BASE/releases"
ls -1t | tail -n +$((KEEP_RELEASES + 1)) | while read -r old; do
    [[ "$old" =~ ^[0-9a-f]{40}$ && "$old" != "$release_id" && "$BASE/releases/$old" != "$live_release" ]] || continue
    rm -rf -- "${BASE:?}/releases/$old"
done

# Fingerprinted files are kept for every release still on disk.
keep="$(mktemp)"
for kept in "$BASE"/releases/*/; do
    [[ -d "$kept/assets" ]] && ( cd "$kept/assets" && find . -type f ! -name manifest.json ) >>"$keep"
done
sort -u "$keep" -o "$keep"
if [[ -s "$keep" && -d "$STATIC/assets" ]]; then
    ( cd "$STATIC/assets" && find . -type f ) | sort | comm -23 - "$keep" | while read -r stale; do
        rm -f -- "$STATIC/assets/$stale"
    done
    find "$STATIC/assets" -mindepth 1 -type d -empty -delete 2>/dev/null || true
fi
rm -f "$keep"

echo "Release ${release_id:0:7} is live on port $idle_port."
