const fs = require('fs');
const path = require('path');

const binaries = {
  linux: 'cli-linux',
  darwin: 'cli-macos',
  win32: 'cli-win.exe'
};

const platform = process.platform;
const binary = binaries[platform];

if (!binary) {
  console.error('Unsupported platform:', platform);
  process.exit(1);
}

const binaryPath = path.join(__dirname, '..', 'bin', binary);
if (!fs.existsSync(binaryPath)) {
  console.error('Binary not found:', binaryPath);
  process.exit(1);
}

console.log('Binary setup complete:', binary);
