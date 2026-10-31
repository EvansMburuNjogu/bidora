#!/bin/sh
set -eu
case "${RENEWED_LINEAGE:-}" in
  /etc/letsencrypt/live/bidora-sandbox)
    nginx -t
    systemctl reload nginx
    ;;
esac
