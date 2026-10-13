#!/bin/sh
set -e
systemctl daemon-reload || true
systemctl enable neohub.service >/dev/null 2>&1 || true
systemctl enable neohub-mdns.service >/dev/null 2>&1 || true
systemctl restart neohub-mdns.service >/dev/null 2>&1 || true
# Start on a fresh install, restart on an upgrade.
if systemctl is-active --quiet neohub.service; then
  systemctl restart neohub.service || true
else
  systemctl start neohub.service || true
fi
host="$(hostname 2>/dev/null || echo raspberrypi)"
echo ""
echo "neoHub is running. Open it from any device on your network:"
echo "    http://neohub.local/   (or http://${host}.local:8484/)"
echo "Run 'neohub status' to see the hub it found, or 'neohub qr' for a phone QR code."
echo ""
