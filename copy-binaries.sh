#!/usr/bin/env bash

set -e

echo "Copying built binaries to bin/..."

# Copy Linux binary
if [ -f dist/nextgen-go-cli_linux_amd64_v1/nextgen-go-cli ]; then
  cp dist/nextgen-go-cli_linux_amd64_v1/nextgen-go-cli bin/cli-linux
  chmod +x bin/cli-linux
  echo "✔ Copied Linux binary to bin/cli-linux"
else
  echo "✘ Missing dist/nextgen-go-cli_linux_amd64_v1/nextgen-go-cli"
fi

# Copy macOS binary
if [ -f dist/nextgen-go-cli_darwin_amd64_v1/nextgen-go-cli ]; then
  cp dist/nextgen-go-cli_darwin_amd64_v1/nextgen-go-cli bin/cli-macos
  chmod +x bin/cli-macos
  echo "✔ Copied macOS binary to bin/cli-macos"
else
  echo "✘ Missing dist/nextgen-go-cli_darwin_amd64_v1/nextgen-go-cli"
fi

# Copy Windows binary
if [ -f dist/nextgen-go-cli_windows_amd64_v1/nextgen-go-cli.exe ]; then
  cp dist/nextgen-go-cli_windows_amd64_v1/nextgen-go-cli.exe bin/cli-win.exe
  chmod +x bin/cli-win.exe || true  # May fail on Windows
  echo "✔ Copied Windows binary to bin/cli-win.exe"
else
  echo "✘ Missing dist/nextgen-go-cli_windows_amd64_v1/nextgen-go-cli.exe"
fi

echo "Done copying binaries." 