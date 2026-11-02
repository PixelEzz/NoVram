#!/usr/bin/env bash
# novram cross-platform build & packaging script
# Generates zero-dependency static binaries and release archives in ./dist

set -euo pipefail

VERSION="${1:-0.1.3}"
DIST_DIR="$(pwd)/dist"

echo "=========================================================="
echo " novram Cross-Platform Build Automation (v${VERSION})"
echo "=========================================================="

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

export CGO_ENABLED=0
LDFLAGS="-s -w -X main.Version=${VERSION}"

# Target matrices: (OS ARCH EXT ARCHIVE_TYPE)
TARGETS=(
  "windows amd64 .exe zip"
  "linux amd64 "" tar.gz"
  "linux arm64 "" tar.gz"
  "darwin arm64 "" tar.gz"
  "darwin amd64 "" tar.gz"
)

for target in "${TARGETS[@]}"; do
  read -r GOOS GOARCH EXT ARCHIVE_TYPE <<< "${target}"
  BIN_NAME="novram-${GOOS}-${GOARCH}${EXT}"
  BIN_PATH="${DIST_DIR}/${BIN_NAME}"

  echo "--> Compiling ${GOOS}/${GOARCH} -> ${BIN_NAME}..."
  GOOS="${GOOS}" GOARCH="${GOARCH}" go build -ldflags="${LDFLAGS}" -o "${BIN_PATH}" ./cmd/novram

  cd "${DIST_DIR}"
  if [ "${ARCHIVE_TYPE}" = "zip" ]; then
    ARCHIVE_NAME="novram-${GOOS}-${GOARCH}.zip"
    echo "    Packaging ${ARCHIVE_NAME}..."
    if command -v zip >/dev/null 2>&1; then
      zip -q -9 "${ARCHIVE_NAME}" "${BIN_NAME}"
    else
      tar -czf "${ARCHIVE_NAME}" "${BIN_NAME}"
    fi
  else
    ARCHIVE_NAME="novram-${GOOS}-${GOARCH}.tar.gz"
    echo "    Packaging ${ARCHIVE_NAME}..."
    tar -czf "${ARCHIVE_NAME}" "${BIN_NAME}"
  fi

  # Generate individual sha256
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${ARCHIVE_NAME}" > "${ARCHIVE_NAME}.sha256"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${ARCHIVE_NAME}" > "${ARCHIVE_NAME}.sha256"
  fi
  cd - >/dev/null
done

# Generate consolidated checksums.txt
echo "--> Generating dist/checksums.txt..."
cd "${DIST_DIR}"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum novram-*.* > checksums.txt
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 novram-*.* > checksums.txt
fi
cd - >/dev/null

echo "=========================================================="
echo " Build & packaging complete! Artifacts in: ./dist"
ls -la "${DIST_DIR}"
echo "=========================================================="
