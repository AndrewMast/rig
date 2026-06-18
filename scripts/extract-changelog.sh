#!/usr/bin/env bash
# Print the CHANGELOG.md section for a given version (without the leading "v").
# Usage: scripts/extract-changelog.sh 0.1.0
set -euo pipefail

version="${1:?usage: extract-changelog.sh <version>}"

awk -v ver="$version" '
  $0 ~ "^## \\[" ver "\\]" { grab = 1; print; next }
  grab && /^## \[/ { exit }
  grab { print }
' CHANGELOG.md
