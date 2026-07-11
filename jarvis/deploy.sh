#!/bin/sh -e
# Build and deploy JARVIS to its launchd runtime home.
# macOS TCC blocks launchd agents from ~/Documents, so the running copy
# lives in ~/Library/JARVIS (binary + .env + memory). Repo stays the source.
cd "$(dirname "$0")"
go build -o jarvis .
go test ./... > /dev/null
mkdir -p "$HOME/Library/JARVIS"
cp jarvis "$HOME/Library/JARVIS/jarvis"
cp .env "$HOME/Library/JARVIS/.env"
launchctl kickstart -k "gui/$(id -u)/com.noxioai.jarvis.serve" 2>/dev/null || true
echo "✓ built, tested, deployed — HUD restarted on http://127.0.0.1:7700"
