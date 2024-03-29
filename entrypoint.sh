#!/usr/bin/env bash
set -o errexit

PORT="${PORT:-5000}"
HOST="${HOST:-imgs.sh}"

echo -e "$PRIVATE_KEY" > ./key
chmod 600 ./key

exec autossh -M 0 -N \
  -L 0.0.0.0:"$PORT":localhost:"$PORT" \
  -o IdentitiesOnly=yes \
  -o StrictHostKeyChecking=no \
  -i ./key \
  "$USERNAME"@"$HOST"
