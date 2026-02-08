#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

const VERSION = process.argv[2];

if (!VERSION) {
  console.error('Error: Version not provided');
  console.error('Usage: node update-version.js <version>');
  process.exit(1);
}

// Validate version format (basic semver check)
if (!/^\d+\.\d+\.\d+$/.test(VERSION)) {
  console.error('Error: Version must be in format X.Y.Z (e.g., 1.0.0)');
  process.exit(1);
}

console.log(`Updating version to ${VERSION}...`);

// Update version.json
const versionJsonPath = path.join(__dirname, 'version.json');
const versionJson = {
  version: VERSION
};
fs.writeFileSync(versionJsonPath, JSON.stringify(versionJson, null, 4) + '\n');
console.log('✓ Updated version.json');

// Update types.go
const typesGoPath = path.join(__dirname, 'src', 'types.go');
let typesGoContent = fs.readFileSync(typesGoPath, 'utf8');
typesGoContent = typesGoContent.replace(
  /APP_VERSION\s*=\s*"[^"]+"/,
  `APP_VERSION = "${VERSION}"`
);
fs.writeFileSync(typesGoPath, typesGoContent);
console.log('✓ Updated src/types.go');

console.log(`\nVersion successfully updated to ${VERSION}`);
