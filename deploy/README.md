# Disaster Recovery — rebuild the NOXIOAI server from zero

Current production: **95.181.160.117** (aeza DE, Ubuntu 26.04, expires 2026-08-24).
If that box is lost, a fresh Ubuntu VPS becomes production in ~20 minutes:

## 1. Access
```sh
ssh-copy-id root@NEW_IP          # your key, password once
```

## 2. Harden (before anything else)
```sh
scp deploy/sshd-00-hardening.conf root@NEW_IP:/etc/ssh/sshd_config.d/00-hardening.conf
ssh root@NEW_IP 'apt-get update -qq && apt-get install -y ufw fail2ban unattended-upgrades && \
  ufw allow OpenSSH && ufw allow 80/tcp && ufw allow 443/tcp && ufw --force enable && \
  systemctl enable --now fail2ban unattended-upgrades && sshd -t && systemctl reload ssh'
```
NB: the hardening file must sort BEFORE `50-cloud-init.conf` (sshd: first value wins).

## 3. Provision app + database
```sh
# build binary from repo root (any machine with Go):
cd jarvis && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/jarvis-linux . && cd ..
ssh root@NEW_IP 'mkdir -p /root/staging'
scp /tmp/jarvis-linux deploy/provision-jarvis.sh root@NEW_IP:/root/staging/
# .env: restore from your password manager / local copy (never in git)
scp jarvis/.env root@NEW_IP:/root/staging/.env
ssh root@NEW_IP 'bash /root/staging/provision-jarvis.sh'   # postgres, user, systemd timers, tz
```

## 4. Restore data
Latest encrypted dump is in your Telegram (bot posts nightly 03:30).
Passphrase: in your password manager (originally `/opt/jarvis/.backup-pass`).
```sh
openssl enc -d -aes-256-cbc -pbkdf2 -in jarvis-YYYYMMDD.dump.enc -out restore.dump -pass pass:PASSPHRASE
scp restore.dump root@NEW_IP:/tmp/ && ssh root@NEW_IP \
  'sudo -u postgres pg_restore --clean --if-exists --no-owner --role=jarvis -d jarvis /tmp/restore.dump && rm /tmp/restore.dump'
```

## 5. Web + DNS
```sh
scp deploy/Caddyfile root@NEW_IP:/etc/caddy/Caddyfile
ssh root@NEW_IP 'apt-get install -y caddy && systemctl enable --now caddy'
```
Cloudflare: point `api` A record to NEW_IP (DNS only / grey cloud — Caddy does its own TLS).

## 6. Verify
```sh
ssh root@NEW_IP 'systemctl list-timers "jarvis-*" --no-pager; systemctl is-active jarvis postgresql caddy'
curl https://api.noxioai.com/api/auth/me   # expect 401
```

## Known constraints
- **aeza blocks outbound SMTP (25/465/587)** — email must go through an HTTPS ESP API, never net/smtp from the box.
- Never send cold email from the server IP even if ports were open (datacenter IP = spam).
- Frontend is NOT on this server (GitHub Pages → moving to Vercel); this box is API + JARVIS + Postgres only.
