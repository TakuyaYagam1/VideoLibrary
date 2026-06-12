#!/usr/bin/env sh
set -eu

assets_dir="${ASSETS_DIR:-../assets}"
seaweedfs_url="${SEAWEEDFS_FILER_URL:-http://seaweedfs:${SEAWEEDFS_PORT:-8888}}"
video_prefix="${SEAWEEDFS_VIDEO_PREFIX:-videos}"
retry_count="${SEAWEEDFS_UPLOAD_RETRIES:-30}"
retry_delay="${SEAWEEDFS_UPLOAD_RETRY_DELAY:-1}"
asset_files="${SEAWEEDFS_ASSET_FILES:-planet_1.5mb.mp4 planet_3mb.mp4 planet_10mb.mp4 planet_18mb.mp4}"

wait_for_filer() {
  url="${seaweedfs_url%/}/"

  attempt=1
  while [ "${attempt}" -le "${retry_count}" ]; do
    if curl --fail --silent --show-error --output /dev/null "${url}"; then
      return 0
    fi

    sleep "${retry_delay}"
    attempt=$((attempt + 1))
  done

  echo "SeaweedFS filer is not ready: ${url}" >&2
  return 1
}

upload_file() {
  source_path="$1"
  target_url="$2"

  attempt=1
  while [ "${attempt}" -le "${retry_count}" ]; do
    if curl --fail --silent --show-error \
      --request PUT \
      --header "Content-Type: video/mp4" \
      --data-binary @"${source_path}" \
      "${target_url}" >/dev/null; then
      return 0
    fi

    sleep "${retry_delay}"
    attempt=$((attempt + 1))
  done

  echo "failed to upload ${source_path} -> ${target_url}" >&2
  return 1
}

wait_for_filer

for file in ${asset_files}; do
  source_path="${assets_dir%/}/${file}"
  target_url="${seaweedfs_url%/}/${video_prefix#/}/${file}"

  if [ ! -f "${source_path}" ]; then
    echo "missing asset: ${source_path}" >&2
    exit 1
  fi

  upload_file "${source_path}" "${target_url}"

  echo "uploaded ${source_path} -> ${target_url}"
done
