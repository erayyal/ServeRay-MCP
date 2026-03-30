#!/usr/bin/env sh
set -eu

SERVER="${1:-}"
DIST_DIR="${DIST_DIR:-dist}"
VERSION="${VERSION:-snapshot}"

if [ -z "$SERVER" ]; then
  echo "usage: ./scripts/build-release.sh <server-name>" >&2
  exit 1
fi

if [ ! -d "./cmd/$SERVER" ]; then
  echo "unknown server: $SERVER" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go is required on PATH for packaging" >&2
  exit 1
fi

mkdir -p "$DIST_DIR"
DIST_ABS="$(cd "$DIST_DIR" && pwd)"
STAGE_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/${SERVER}.XXXXXX")"
CHECKSUM_FILE="$DIST_ABS/${SERVER}_${VERSION}_checksums.txt"

cleanup() {
  rm -rf "$STAGE_ROOT"
}

trap cleanup EXIT HUP INT TERM

: > "$CHECKSUM_FILE"

sha256_line() {
  FILE_PATH="$1"
  FILE_NAME="$(basename "$FILE_PATH")"

  if command -v sha256sum >/dev/null 2>&1; then
    HASH="$(sha256sum "$FILE_PATH" | cut -d' ' -f1)"
  elif command -v shasum >/dev/null 2>&1; then
    HASH="$(shasum -a 256 "$FILE_PATH" | cut -d' ' -f1)"
  elif command -v openssl >/dev/null 2>&1; then
    HASH="$(openssl dgst -sha256 -r "$FILE_PATH" | cut -d' ' -f1)"
  else
    echo "sha256 tool not found; install sha256sum, shasum, or openssl" >&2
    exit 1
  fi

  printf '%s  %s\n' "$HASH" "$FILE_NAME" >> "$CHECKSUM_FILE"
}

build_one() {
  GOOS="$1"
  GOARCH="$2"
  EXT="$3"
  ARCHIVE_BASENAME="${SERVER}_${VERSION}_${GOOS}_${GOARCH}"
  OUT_SUBDIR="$STAGE_ROOT/$ARCHIVE_BASENAME"

  mkdir -p "$OUT_SUBDIR"
  GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -o "$OUT_SUBDIR/$SERVER$EXT" "./cmd/$SERVER"
  cp LICENSE "$OUT_SUBDIR/"
  cp "./cmd/$SERVER/README.md" "$OUT_SUBDIR/README.md"
  cp "./cmd/$SERVER/.env.example" "$OUT_SUBDIR/.env.example"

  if [ "$GOOS" = "windows" ]; then
    if ! command -v zip >/dev/null 2>&1; then
      echo "zip is required to package Windows archives" >&2
      exit 1
    fi
    ARCHIVE_PATH="$DIST_ABS/${ARCHIVE_BASENAME}.zip"
    (
      cd "$OUT_SUBDIR"
      zip -q -r "$ARCHIVE_PATH" .
    )
  else
    ARCHIVE_PATH="$DIST_ABS/${ARCHIVE_BASENAME}.tar.gz"
    tar -C "$OUT_SUBDIR" -czf "$ARCHIVE_PATH" .
  fi

  sha256_line "$ARCHIVE_PATH"
}

build_one darwin amd64 ""
build_one darwin arm64 ""
build_one linux amd64 ""
build_one linux arm64 ""
build_one windows amd64 ".exe"
build_one windows arm64 ".exe"

echo "packaged $SERVER into $DIST_ABS"
echo "wrote checksums to $CHECKSUM_FILE"
