#!/usr/bin/env sh
set -eu

SERVER="${1:-}"
PREFIX="${PREFIX:-$HOME/.local/bin}"

if [ -z "$SERVER" ]; then
  echo "usage: ./scripts/install.sh <server-name>" >&2
  exit 1
fi

if [ ! -d "./cmd/$SERVER" ]; then
  echo "unknown server: $SERVER" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go is required on PATH for source installation" >&2
  exit 1
fi

mkdir -p "$PREFIX"
go build -trimpath -o "$PREFIX/$SERVER" "./cmd/$SERVER"
echo "installed $SERVER to $PREFIX/$SERVER"
