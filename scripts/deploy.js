const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const packageJsonPath = path.resolve(__dirname, '..', 'package.json');
const mainGoPath = path.resolve(__dirname, '..', 'main.go');

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
  const newVersionTag = `v${newVersion}`;
  console.log(`New version: ${newVersion}`);

  // --- 3. Update package.json ---
  packageData.version = newVersion;
  fs.writeFileSync(packageJsonPath, JSON.stringify(packageData, null, 2) + '\n', 'utf8'); // Ensure trailing newline
  console.log('Updated package.json with new version.');

  // --- 4. Update main.go ---
  console.log('Updating main.go...');
  let mainGoContent = fs.readFileSync(mainGoPath, 'utf8');
  const versionRegex = /(var\s+Version\s*=\s*")v?[^"\s]*(")/; 
  if (!versionRegex.test(mainGoContent)) {
      throw new Error(`Could not find 'var Version = "..."' line in ${mainGoPath}`);
  }
  mainGoContent = mainGoContent.replace(versionRegex, `$1${newVersionTag}$2`);
  fs.writeFileSync(mainGoPath, mainGoContent, 'utf8');
  console.log(`Updated main.go with new version: ${newVersionTag}`);

  // --- 5. Git Commit ---
  try {
    console.log('Committing version bump...');
    execSync(`git add ${packageJsonPath} ${mainGoPath}`, { stdio: 'inherit' });
    execSync(`git commit -m "chore: Release ${newVersionTag}"`, { stdio: 'inherit' });
    console.log('Committed staged changes for release.');
  } catch (commitError) {
    console.warn(`Warning: Failed to commit release changes. You might need to commit manually. Error: ${commitError.message}`);
  }

  // --- 6. Git Tag ---
  const tagName = newVersionTag;
  console.log(`Creating git tag: ${tagName}...`);
  execSync(`git tag ${tagName}`, { stdio: 'inherit' });
  console.log(`Successfully created git tag: ${tagName}`);

  // --- 7. Git Push Tag ---
  console.log(`Pushing tag ${tagName} to origin...`);
  execSync(`git push origin ${tagName}`, { stdio: 'inherit' });
  console.log(`Successfully pushed tag ${tagName} to origin.`);

  console.log('\nDeployment script finished successfully!');

} catch (error) {
  console.error(`\nError during deployment script: ${error.message}`);
  process.exit(1); // Exit with error code
} 