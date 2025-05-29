#!/bin/bash
# filepath: update_versions.sh

# Find all files named "version" in the git directory
VERSION_FILES=$(find . -name "version" -type f)

# Check if any version files were found
if [ -z "$VERSION_FILES" ]; then
  echo "No 'version' files found in the repository"
  exit 1
fi

echo "Found the following version files:"
echo "$VERSION_FILES"
echo "------------------------"

# Process each version file
for file in $VERSION_FILES; do
  echo "Processing $file"

  # Read the current version
  CURRENT_VERSION=$(cat "$file")
  echo "Current version: $CURRENT_VERSION"

  # Check if it follows the vX.Y format
  if [[ $CURRENT_VERSION =~ ^v([0-9]+)\.([0-9]+)$ ]]; then
    MAJOR="${BASH_REMATCH[1]}"
    MINOR="${BASH_REMATCH[2]}"

    # Increment the minor version
    NEW_MINOR=$((MINOR + 1))
    NEW_VERSION="v$MAJOR.$NEW_MINOR"

    # Update the file
    echo "$NEW_VERSION" > "$file"
    echo "Updated to: $NEW_VERSION"
  else
    echo "WARNING: $file does not contain a version in the expected format (vX.Y). Skipping."
  fi

  echo "------------------------"
done

echo "Version update completed"
