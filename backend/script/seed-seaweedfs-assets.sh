#!/usr/bin/env bash
set -euo pipefail

assets_dir="${ASSETS_DIR:-../assets}"
seaweedfs_url="${SEAWEEDFS_FILER_URL:-${SEAWEEDFS_PUBLIC_URL:-http://localhost:8888}}"
video_prefix="${SEAWEEDFS_VIDEO_PREFIX:-videos}"

files=(
  "planet_1.5mb.mp4"
  "planet_3mb.mp4"
  "planet_10mb.mp4"
  "planet_18mb.mp4"
)

for file in "${files[@]}"; do
  source_path="${assets_dir%/}/${file}"
  target_url="${seaweedfs_url%/}/${video_prefix#/}/${file}"

  if [[ ! -f "${source_path}" ]]; then
    echo "missing asset: ${source_path}" >&2
    exit 1
  fi

  curl --fail --silent --show-error \
    --request PUT \
    --header "Content-Type: video/mp4" \
    --data-binary @"${source_path}" \
    "${target_url}" >/dev/null

  echo "uploaded ${source_path} -> ${target_url}"
done
