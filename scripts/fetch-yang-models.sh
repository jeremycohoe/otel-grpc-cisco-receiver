#!/bin/bash
# Download Cisco IOS XE YANG models from github.com/YangModels/yang
#
# Usage:
#   ./scripts/fetch-yang-models.sh          # Downloads default version (17181)
#   ./scripts/fetch-yang-models.sh 17151    # Downloads specific version
#   ./scripts/fetch-yang-models.sh 17181 /custom/path  # Custom output directory
#
# Only downloads .yang files from the top-level directory (skips MIBS, BIC subfolders).

set -euo pipefail

VERSION="${1:-17181}"
OUTPUT_DIR="${2:-$(cd "$(dirname "$0")/.." && pwd)/yang-models}"
REPO="YangModels/yang"
BRANCH="main"
GITHUB_PATH="vendor/cisco/xe/${VERSION}"
API_URL="https://api.github.com/repos/${REPO}/contents/${GITHUB_PATH}?ref=${BRANCH}"

echo "Fetching Cisco IOS XE YANG models..."
echo "  Version: ${VERSION}"
echo "  Source:  github.com/${REPO}/tree/${BRANCH}/${GITHUB_PATH}"
echo "  Output:  ${OUTPUT_DIR}"
echo ""

mkdir -p "${OUTPUT_DIR}"

# Get directory listing from GitHub API (top-level only, no subfolders)
echo "Querying GitHub API for file list..."
file_list=$(curl -sf "${API_URL}" 2>/dev/null) || {
  echo "ERROR: Failed to fetch file list from GitHub."
  echo "  Check: https://github.com/${REPO}/tree/${BRANCH}/${GITHUB_PATH}"
  echo "  The version '${VERSION}' may not exist."
  exit 1
}

# Extract .yang file download URLs (top-level only, type=file)
download_urls=$(echo "${file_list}" | python3 -c "
import sys, json
items = json.load(sys.stdin)
for item in items:
    if item['type'] == 'file' and item['name'].endswith('.yang'):
        print(item['download_url'])
")

total=$(echo "${download_urls}" | wc -l)
echo "Found ${total} .yang files to download"
echo ""

# Download each file
count=0
for url in ${download_urls}; do
  filename=$(basename "${url}")
  count=$((count + 1))

  if [[ -f "${OUTPUT_DIR}/${filename}" ]]; then
    continue
  fi

  printf "\r  [%d/%d] %s" "${count}" "${total}" "${filename}"
  curl -sf -o "${OUTPUT_DIR}/${filename}" "${url}" || {
    echo ""
    echo "WARNING: Failed to download ${filename}"
  }
done

echo ""
echo ""

downloaded=$(find "${OUTPUT_DIR}" -maxdepth 1 -name '*.yang' | wc -l)
echo "Done: ${downloaded} YANG files in ${OUTPUT_DIR}"
echo ""
echo "Configure the receiver to use them:"
echo "  cisco_telemetry:"
echo "    yang:"
echo "      models_dir: ${OUTPUT_DIR}"
