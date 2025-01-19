#!/usr/bin/env node

// tag-and-push-version.js

const { execSync } = require("child_process");
const fs = require("fs");

// Read version from package.json
const pkg = JSON.parse(fs.readFileSync("./package.json", "utf8"));
const version = pkg.version;

try {
  // Stage all changes
  execSync("git add .", { stdio: "inherit" });
  
  // Commit changes with version in the message
  execSync(`git commit -m "chore: release v${version}"`, { stdio: "inherit" });
  
  // Create a tag based on package.json version
  execSync(`git tag v${version}`, { stdio: "inherit" });
  
  // Push both the commit(s) and the tag
  execSync("git push origin HEAD && git push origin --tags", { stdio: "inherit" });
  
  console.log(`Successfully tagged and pushed v${version}.`);
} catch (err) {
  console.error("Failed to tag or push:", err);
  process.exit(1);
} 