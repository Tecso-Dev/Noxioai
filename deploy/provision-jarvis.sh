#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

echo "--- postgres install ---"
apt-get install -y -qq postgresql postgresql-client >/dev/null
systemctl enable --now postgresql --quiet

echo "--- jarvis system user ---"
id -u jarvis >/dev/null 2>&1 || useradd --system --create-home --home-dir /opt/jarvis --shell /usr/sbin/nologin jarvis
mkdir -p /opt/jarvis/memory

echo "--- database + role (password never printed) ---"
DBPW=$(openssl rand -hex 24)
sudo -u postgres psql -qtc "SELECT 1 FROM pg_roles WHERE rolname='jarvis'" | grep -q 1 \
  || sudo -u postgres psql -qc "CREATE ROLE jarvis LOGIN"
sudo -u postgres psql -qc "ALTER ROLE jarvis LOGIN PASSWORD '$DBPW'"
sudo -u postgres psql -qtc "SELECT 1 FROM pg_database WHERE datname='jarvis'" | grep -q 1 \
  || sudo -u postgres createdb -O jarvis jarvis

echo "--- restore dump ---"
sudo -u postgres pg_restore --clean --if-exists --no-owner --role=jarvis -d jarvis /root/staging/jarvis-db.dump 2>&1 | grep -v "^$" | head -3 || true
sudo -u postgres psql -qc "GRANT ALL ON ALL TABLES IN SCHEMA public TO jarvis; GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO jarvis;" -d jarvis

echo "--- files ---"
install -m 755 /root/staging/jarvis-linux /opt/jarvis/jarvis
tar -xzf /root/staging/memory.tgz -C /opt/jarvis/memory --strip-components=1 2>/dev/null || echo "(no memory files)"
cp /root/staging/.env /opt/jarvis/.env

echo "--- env adaptation ---"
if grep -q '^JARVIS_DB_URL=' /opt/jarvis/.env; then
  sed -i "s|^JARVIS_DB_URL=.*|JARVIS_DB_URL=postgres://jarvis:$DBPW@localhost:5432/jarvis?sslmode=disable|" /opt/jarvis/.env
else
  echo "JARVIS_DB_URL=postgres://jarvis:$DBPW@localhost:5432/jarvis?sslmode=disable" >> /opt/jarvis/.env
fi
sed -i "s|^APP_BASE_URL=.*|APP_BASE_URL=https://noxioai.com|" /opt/jarvis/.env
if grep -q '^JARVIS_MEMORY_DIR=' /opt/jarvis/.env; then
  sed -i "s|^JARVIS_MEMORY_DIR=.*|JARVIS_MEMORY_DIR=/opt/jarvis/memory|" /opt/jarvis/.env
else
  echo "JARVIS_MEMORY_DIR=/opt/jarvis/memory" >> /opt/jarvis/.env
fi
chown -R jarvis:jarvis /opt/jarvis
chmod 600 /opt/jarvis/.env

echo "--- timezone ---"
timedatectl set-timezone Europe/Warsaw

echo "--- systemd units ---"
cat > /etc/systemd/system/jarvis-brief.service <<'EOF'
[Unit]
Description=JARVIS morning briefing
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=oneshot
User=jarvis
WorkingDirectory=/opt/jarvis
ExecStart=/opt/jarvis/jarvis brief
EOF

cat > /etc/systemd/system/jarvis-brief.timer <<'EOF'
[Unit]
Description=Daily JARVIS briefing at 08:00

[Timer]
OnCalendar=*-*-* 08:00:00
Persistent=true

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable --now jarvis-brief.timer --quiet

echo "--- verification ---"
sudo -u postgres psql -d jarvis -qtc "select 'db-leads:'||count(*) from leads;"
systemctl list-timers jarvis-brief.timer --no-pager | head -3
rm -rf /root/staging
echo "PROVISION-DONE"
