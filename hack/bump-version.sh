#!/bin/bash

# Function to bump minor version
bump_minor_version() {
    version_str=$1
    major=$(echo "$version_str" | cut -d. -f1)
    minor=$(echo "$version_str" | cut -d. -f2)
    new_minor=$((minor + 1))
    echo "${major}.${new_minor}"
}

# Use the first argument as the base directory, default to current directory if not provided
base_directory=${1:-.}

# Find command wrapped in a condition to handle the case where no files are found
find "$base_directory" -name 'version' | while IFS= read -r version_file; do
    if [ -f "$version_file" ]; then
        current_version=$(cat "$version_file")
        new_version=$(bump_minor_version "$current_version")
        echo "$new_version" > "$version_file"
        echo "Updated $version_file: $current_version -> $new_version"
    fi
done