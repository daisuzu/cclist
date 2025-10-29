#!/bin/bash
set -e

# Remove all test data except scripts and README
find . -mindepth 1 -maxdepth 1 \
  ! -name 'README.md' \
  ! -name 'reset.sh' \
  ! -name '.gitignore' \
  -exec rm -rf {} +

# Create .cclist directory and empty config
mkdir -p .cclist
echo '{}' > .cclist/config.json

# aaa: repository with worktree
mkdir -p aaa/.claude
cd aaa
git init --quiet
git config user.name "Test User"
git config user.email "test@example.com"
echo "# Repository A" > README.md
git add README.md
git commit -m "Initial commit" --quiet
git worktree add ../aaa-feature -b feature --quiet
cd ../aaa-feature
mkdir -p .claude
echo "# Feature Branch" > FEATURE.md
git add FEATURE.md
git commit -m "Add feature" --quiet
cd ..

# bbb: repository without worktree
mkdir -p bbb/.claude
cd bbb
git init --quiet
git config user.name "Test User"
git config user.email "test@example.com"
echo "# Repository B" > README.md
git add README.md
git commit -m "Initial commit" --quiet
cd ..

# ccc: repository without .claude/ directory
mkdir -p ccc
cd ccc
git init --quiet
git config user.name "Test User"
git config user.email "test@example.com"
echo "# Repository C" > README.md
git add README.md
git commit -m "Initial commit" --quiet
cd ..

echo "✓ Testdata ready"
