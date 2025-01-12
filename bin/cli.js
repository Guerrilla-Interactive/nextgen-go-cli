#!/usr/bin/env node
const { execFileSync } = require('child_process');
const path = require('path');
const os = require('os');

const platform = os.platform();

let binary;
if (platform === 'darwin') binary = 'nextgen-go-cli_darwin_amd64_v1/nextgen-go-cli';
else if (platform === 'win32') binary = 'nextgen-go-cli_windows_amd64_v1/nextgen-go-cli.exe';
else if (platform === 'linux') binary = 'nextgen-go-cli_linux_amd64_v1/nextgen-go-cli';
else {
  console.error(`Unsupported platform: ${platform}`);
  process.exit(1);
}

const binaryPath = path.resolve(__dirname, '..', 'dist', binary);

try {
  execFileSync(binaryPath, process.argv.slice(2), { stdio: 'inherit' });
} catch (err) {
  console.error(`Failed to execute binary: ${err.message}`);
  process.exit(1);
}
