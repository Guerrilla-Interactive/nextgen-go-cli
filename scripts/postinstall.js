const fs = require('fs');
const path = require('path');
const os = require('os');

const platform = os.platform();
const binaries = {
  linux: 'cli-linux',
  darwin: 'cli-macos',
  win32: 'cli-win.exe',
};

// Determine the expected binary for the current platform
const expectedBinary = binaries[platform];
if (!expectedBinary) {
  console.error(`Unsupported platform: ${platform}`);
  process.exit(1);
}

// Construct the binary path
const binaryPath = path.resolve(__dirname, '..', 'bin', expectedBinary);

// Check if the binary exists
if (fs.existsSync(binaryPath)) {
  console.log(`CLI binary is ready for platform: ${platform}`);
} else {
  console.warn(`Warning: Binary missing for platform ${platform}: ${expectedBinary}`);
  console.warn(
    `Ensure the \`bin/\` directory contains the required binary: ${expectedBinary}`
  );
  console.warn(
    'You may need to rebuild the binaries using your build scripts or contact the package maintainer.'
  );
  process.exit(1); // Exit with error to indicate missing binary
}

// Optional: Check all binaries and log their statuses
console.log('Checking all binaries:');
Object.values(binaries).forEach((binary) => {
  const binaryFilePath = path.resolve(__dirname, '..', 'bin', binary);
  if (fs.existsSync(binaryFilePath)) {
    console.log(`✔ Found: ${binary}`);
  } else {
    console.warn(`✘ Missing: ${binary}`);
  }
});
