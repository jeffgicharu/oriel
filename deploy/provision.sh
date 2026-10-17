#!/usr/bin/env bash
#
# Prepares the server for Oriel. Run once, as deploy, from a temporary folder
# (made with mktemp -d) that holds the files it installs:
#
#   nginx.conf  oriel@.service  activate.sh  unavailable.html
#   deploy-keys.pub  desk-entry-key
#
# The folder is sent and unpacked in the same SSH connection that runs this:
#
#   tar -czf - -C <folder> . | ssh deploy@<server> \
#     'd="$(mktemp -d)" && { tar -xzf - -C "$d" && bash "$d/provision.sh"; rc=$?; rm -rf -- "$d"; exit $rc; }'
#
# The folder holds a key, so it is removed when this script ends, whether it
# finishes or fails, and again by the command above if unpacking fails. It is
# safe to run again: anything that already exists is left alone, and no
# secret is ever printed.

set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
BASE=/var/www/oriel.jeffgicharu.com

case "$HERE" in
    /tmp/tmp.*) trap 'rm -rf -- "$HERE"' EXIT ;;
    *) echo "Run this from a folder made by mktemp -d, so it can be removed afterwards." >&2; exit 64 ;;
esac

step() { printf '\n== %s\n' "$1"; }

step "Application user"
if id oriel >/dev/null 2>&1; then
    echo "oriel exists"
else
    sudo useradd --system --home-dir /nonexistent --no-create-home --shell /usr/sbin/nologin oriel
    echo "created oriel"
fi

step "Folders"
sudo install -d -o deploy -g deploy -m 755 "$BASE" "$BASE/releases" "$BASE/slots" "$BASE/shared" "$BASE/shared/static" "$BASE/shared/pages"
# The databases: the shared sample and every visitor's copy. Only the app writes here.
sudo install -d -o oriel -g oriel -m 750 "$BASE/shared/data"
cp "$HERE/unavailable.html" "$BASE/shared/pages/unavailable.html"
ls -ld "$BASE" "$BASE"/* "$BASE"/shared/*

step "Settings"
if sudo test -f "$BASE/shared/env"; then
    echo "settings file exists; left as it is"
else
    desk_key="$(tr -d '\n' < "$HERE/desk-entry-key")"
    # Created with its final owner and mode before anything is written into it.
    sudo install -m 640 -o root -g oriel /dev/null "$BASE/shared/env"
    {
        echo "ORIEL_SITE_URL=https://oriel.jeffgicharu.com"
        echo "ORIEL_SECRET=$(openssl rand -hex 32)"
        echo "ORIEL_DESK_KEY=$desk_key"
    } | sudo tee -a "$BASE/shared/env" >/dev/null
    unset desk_key
    echo "wrote the settings file"
fi
sudo ls -l "$BASE/shared/env"
echo "settings names: $(sudo cut -d= -f1 "$BASE/shared/env" | tr '\n' ' ')"

step "systemd unit"
sudo install -o root -g root -m 644 "$HERE/oriel@.service" /etc/systemd/system/oriel@.service
sudo systemctl daemon-reload
echo "installed oriel@.service"

step "Activate script"
sudo install -o root -g root -m 755 "$HERE/activate.sh" /usr/local/bin/oriel-activate
ls -l /usr/local/bin/oriel-activate

step "Deploy keys"
touch ~/.ssh/authorized_keys
backup=~/.ssh/authorized_keys.bak-$(date -u +%F)
if [[ ! -e "$backup" ]]; then
    cp -p ~/.ssh/authorized_keys "$backup"
    echo "backed up authorized_keys to $backup"
fi
while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    key="$(awk '{print $(NF-1)}' <<<"$line")"
    if grep -qF "$key" ~/.ssh/authorized_keys; then
        echo "already present: $(awk '{print $NF}' <<<"$line")"
    else
        printf '%s\n' "$line" >> ~/.ssh/authorized_keys
        echo "added: $(awk '{print $NF}' <<<"$line")"
    fi
done < "$HERE/deploy-keys.pub"
echo "authorized_keys lines: $(grep -c . ~/.ssh/authorized_keys)"
awk '/oriel-(upload|activate)$/ {print "line " NR ": " $NF}' ~/.ssh/authorized_keys
kept_lines="$(grep -c . "$backup")"
if cmp -s <(head -n "$kept_lines" ~/.ssh/authorized_keys) "$backup"; then
    echo "lines 1 to $kept_lines match the backup"
else
    echo "lines 1 to $kept_lines do NOT match the backup" >&2
fi

step "nginx"
echo "nginx -t before:"
if ! sudo nginx -t 2>&1 | awk '{print "  " $0}'; then
    echo "nginx does not pass its own test before any change. Stopping without touching it." >&2
    exit 70
fi
sudo install -d -o root -g root -m 755 /etc/nginx/oriel
if [[ ! -f /etc/nginx/oriel/upstream.conf ]]; then
    echo "server 127.0.0.1:8400;" | sudo tee /etc/nginx/oriel/upstream.conf >/dev/null
fi
sudo install -o root -g root -m 644 "$HERE/nginx.conf" /etc/nginx/sites-available/oriel.jeffgicharu.com
sudo ln -sfn /etc/nginx/sites-available/oriel.jeffgicharu.com /etc/nginx/sites-enabled/oriel.jeffgicharu.com
echo "nginx -t after:"
if sudo nginx -t 2>&1 | awk '{print "  " $0}'; then
    sudo systemctl reload nginx
    echo "nginx reloaded with oriel.jeffgicharu.com"
else
    sudo rm -f /etc/nginx/sites-enabled/oriel.jeffgicharu.com
    sudo nginx -t || true
    echo "Oriel's nginx file failed the test and was taken out again." >&2
    exit 1
fi

step "Checks"
command -v rrsync >/dev/null && echo "rrsync: $(command -v rrsync)"
curl -s -o /dev/null -w 'oriel.jeffgicharu.com through nginx before the first release: %{http_code}\n' --insecure --resolve oriel.jeffgicharu.com:443:127.0.0.1 https://oriel.jeffgicharu.com/ || true
free -m | head -2
df -h /var/www | tail -1

echo
echo "Provisioning finished."
