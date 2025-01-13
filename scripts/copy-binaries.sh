#!/usr/bin/env bash

set -e

echo "Copying built binaries to bin/..."

# remove old binaries if they exist
rm -f bin/cli-linux || true
rm -f bin/cli-macos || true
rm -f bin/cli-win.exe || true

# then copy again
cp dist/nextgen-go-cli_linux_amd64_v1/nextgen-go-cli bin/cli-linux || echo "No Linux binary found."
cp dist/nextgen-go-cli_darwin_amd64_v1/nextgen-go-cli bin/cli-macos || echo "No macOS binary found."
cp dist/nextgen-go-cli_windows_amd64_v1/nextgen-go-cli.exe bin/cli-win.exe || echo "No Windows binary found."

chmod +x bin/cli-linux || true
chmod +x bin/cli-macos || true
chmod +x bin/cli-win.exe || true

echo "Done copying binaries." 