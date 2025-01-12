const fs = require('fs');
const path = require('path');

const binaries = ['cli-linux', 'cli-macos', 'cli-win.exe'];
let allBinariesFound = true;

binaries.forEach(binary => {
  const binaryPath = path.resolve(__dirname, '..', 'bin', binary);
  if (!fs.existsSync(binaryPath)) {
    console.warn(`Warning: Binary missing: ${binary}`);
    allBinariesFound = false;
  }
});

if (allBinariesFound) {
  console.log('CLI binaries are ready.');
} else {
  console.warn(
    'Some binaries are missing. Ensure the `bin/` directory is populated with the required binaries.'
  );
}
