#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 <major|minor|patch> <message>"
  echo "  Example: $0 patch \"Fix rate limit detection\""
  exit 1
}

[ $# -lt 2 ] && usage

BUMP=$1
MESSAGE=$2

case "$BUMP" in
  major|minor|patch) ;;
  *) echo "Error: bump type must be major, minor, or patch"; usage ;;
esac

# Get the latest tag; default to v0.0.0 if none exists.
LATEST=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
VERSION=${LATEST#v}  # strip leading 'v'

IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"

case "$BUMP" in
  major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
  patch) PATCH=$((PATCH + 1)) ;;
esac

NEW_TAG="v${MAJOR}.${MINOR}.${PATCH}"

echo "Current: ${LATEST}"
echo "New:     ${NEW_TAG}"
echo "Message: ${MESSAGE}"
echo ""
read -rp "Push tag ${NEW_TAG}? [y/N] " CONFIRM
[[ "$CONFIRM" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 0; }

git tag -a "$NEW_TAG" -m "$MESSAGE"
git push origin "$NEW_TAG"

echo ""
echo "Released ${NEW_TAG}. GitHub Actions will build and publish the release."
