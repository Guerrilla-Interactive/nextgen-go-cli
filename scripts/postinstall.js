const fs = require('fs');
const path = require('path');

const binaries = ['cli-linux', 'cli-macos', 'cli-win.exe'];

binaries.forEach(binary => {
  const binaryPath = path.resolve(__dirname, '..', 'bin', binary);
  if (!fs.existsSync(binaryPath)) {
    console.error(`Binary missing: ${binary}`);
    process.exit(1);
  }
});

console.log('CLI binaries are ready.');
