#!/bin/bash

# Script to generate the anvil skill file from CLI help

set -e

echo "Generating anvil skill file..."

# Build the generator tool
go build -o generate-skill ./scripts/generate-skill/

# Run the generator
./generate-skill

# Clean up
rm generate-skill

echo "Skill file generation complete!"