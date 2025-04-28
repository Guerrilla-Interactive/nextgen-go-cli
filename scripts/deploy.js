const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const packageJsonPath = path.resolve(__dirname, '..', 'package.json');

try {
  // --- 1. Read package.json ---
  console.log('Reading package.json...');
  const packageJsonContent = fs.readFileSync(packageJsonPath, 'utf8');
  const packageData = JSON.parse(packageJsonContent);
  const currentVersion = packageData.version;
  console.log(`Current version: ${currentVersion}`);

  // --- 2. Increment Patch Version ---
  const versionParts = currentVersion.split('.');
  if (versionParts.length !== 3) {
    throw new Error('Invalid version format in package.json. Expected format: X.Y.Z');
  }
  const newPatch = parseInt(versionParts[2], 10) + 1;
  const newVersion = `${versionParts[0]}.${versionParts[1]}.${newPatch}`;
  console.log(`New version: ${newVersion}`);

  // --- 3. Update package.json ---
  packageData.version = newVersion;
  fs.writeFileSync(packageJsonPath, JSON.stringify(packageData, null, 2) + '\n', 'utf8'); // Ensure trailing newline
  console.log('Updated package.json with new version.');

  // --- 4. Git Commit (Optional but Recommended) ---
  // It's generally good practice to commit the version bump.
  try {
    console.log('Committing version bump...');
    execSync(`git add ${packageJsonPath}`, { stdio: 'inherit' });
    execSync(`git commit -m "chore: Release v${newVersion}"`, { stdio: 'inherit' });
    console.log('Committed staged changes for release.');
  } catch (commitError) {
    console.warn(`Warning: Failed to commit release changes. You might need to commit manually. Error: ${commitError.message}`);
    // Decide if you want to proceed without commit or stop. Let's proceed for now.
  }


  // --- 5. Git Tag ---
  const tagName = `v${newVersion}`;
  console.log(`Creating git tag: ${tagName}...`);
  execSync(`git tag ${tagName}`, { stdio: 'inherit' });
  console.log(`Successfully created git tag: ${tagName}`);

  // --- 6. Git Push Tag ---
  console.log(`Pushing tag ${tagName} to origin...`);
  execSync(`git push origin ${tagName}`, { stdio: 'inherit' });
  console.log(`Successfully pushed tag ${tagName} to origin.`);

  console.log('\nDeployment script finished successfully!');

} catch (error) {
  console.error(`\nError during deployment script: ${error.message}`);
  process.exit(1); // Exit with error code
} 