#!/bin/bash
set -euo pipefail
cd /opt/jarvis
# grep vars instead of sourcing: .env is Go-parser format, not always shell-safe
JARVIS_DB_URL=$(grep '^JARVIS_DB_URL=' .env | cut -d= -f2-)
JARVIS_TELEGRAM_CHAT=$(grep '^JARVIS_TELEGRAM_CHAT=' .env | cut -d= -f2-)
JARVIS_TELEGRAM_TOKEN=$(grep '^JARVIS_TELEGRAM_TOKEN=' .env | cut -d= -f2-)
TS=$(date +%Y%m%d-%H%M)
DIR=/opt/jarvis/backups
mkdir -p "$DIR"
F="$DIR/jarvis-$TS.dump"

# 1. local dump (pg custom format, compressed)
pg_dump "$JARVIS_DB_URL" -Fc -f "$F"

# 2. encrypted copy -> Telegram (offsite)
openssl enc -aes-256-cbc -pbkdf2 -salt -in "$F" -out "$F.enc" -pass file:/opt/jarvis/.backup-pass
curl -s -F chat_id="$JARVIS_TELEGRAM_CHAT" -F document=@"$F.enc" \
  -F caption="🗄 jarvis DB backup $TS — aes-256-pbkdf2, decrypt with backup passphrase" \
  "https://api.telegram.org/bot$JARVIS_TELEGRAM_TOKEN/sendDocument" > /dev/null
rm -f "$F.enc"

# 3. rotate: keep 14 local dumps
ls -1t "$DIR"/jarvis-*.dump 2>/dev/null | tail -n +15 | xargs -r rm -f

# 4. integrity check: dump must be restorable-listable
pg_restore -l "$F" > /dev/null && echo "backup OK: $F ($(du -h "$F" | cut -f1))"
