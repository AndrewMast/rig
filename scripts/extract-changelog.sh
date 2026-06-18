#!/usr/bin/env bash
# Print the CHANGELOG.md section for a given version (without the leading "v").
# Usage: scripts/extract-changelog.sh 0.1.0
set -euo pipefail

version="${1:?usage: extract-changelog.sh <version>}"

# Match headers by literal prefix (index) rather than a regex, so this behaves
# identically across awk implementations (gawk, mawk, BSD awk). A dynamic regex
# here silently matched nothing under the CI runner's awk.
awk -v hdr="## [$version]" '
  index($0, hdr) == 1 { grab = 1; print; next }
  grab && index($0, "## [") == 1 { exit }
  grab { print }
' CHANGELOG.md
