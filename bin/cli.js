#!/usr/bin/env node
const { execFileSync } = require('child_process');
const path = require('path');
const os = require('os');

const platform = os.platform();
let binary;

if (platform === 'darwin') binary = 'cli-macos';
else if (platform === 'win32') binary = 'cli-win.exe';
else if (platform === 'linux') binary = 'cli-linux';
else {
  console.error(`Unsupported platform: ${platform}`);
  process.exit(1);
}

const binaryPath = path.resolve(__dirname, binary);

try {
  execFileSync(binaryPath, process.argv.slice(2), { stdio: 'inherit' });
} catch (err) {
  console.error(`Failed to execute binary: ${err.message}`);
  process.exit(1);
}
