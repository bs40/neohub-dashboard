#!/bin/sh
set -e
if [ "$1" = "remove" ] || [ "$1" = "0" ]; then
  systemctl stop neohub.service neohub-mdns.service >/dev/null 2>&1 || true
  systemctl disable neohub.service neohub-mdns.service >/dev/null 2>&1 || true
fi
