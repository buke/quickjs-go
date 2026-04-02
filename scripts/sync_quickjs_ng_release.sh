#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
METADATA_FILE=${METADATA_FILE:-"$ROOT_DIR/deps/quickjs-release.env"}
DEST_DIR=${DEST_DIR:-"$ROOT_DIR/deps/quickjs"}
UPSTREAM_REPO=${UPSTREAM_REPO:-"quickjs-ng/quickjs"}

target_tag=""
force=0

usage() {
    cat <<'EOF'
Usage: sync_quickjs_ng_release.sh [--tag <release-tag>] [--force]

Synchronize deps/quickjs with a quickjs-ng GitHub release tarball.

Options:
  --tag <release-tag>  Sync a specific GitHub release tag instead of the latest release.
  --force              Re-sync even when the requested release matches the vendored tag.
  -h, --help           Show this help message.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --tag)
            if [[ $# -lt 2 ]]; then
                echo "missing value for --tag" >&2
                exit 1
            fi
            target_tag="$2"
            shift 2
            ;;
        --force)
            force=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "unknown argument: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

if [[ ! -f "$METADATA_FILE" ]]; then
    echo "metadata file not found: $METADATA_FILE" >&2
    exit 1
fi

# shellcheck disable=SC1090
source "$METADATA_FILE"

auth_args=()
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    auth_args=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
elif [[ -n "${GH_TOKEN:-}" ]]; then
    auth_args=(-H "Authorization: Bearer ${GH_TOKEN}")
fi

release_api_url="https://api.github.com/repos/${UPSTREAM_REPO}/releases/latest"
if [[ -n "$target_tag" ]]; then
    release_api_url="https://api.github.com/repos/${UPSTREAM_REPO}/releases/tags/${target_tag}"
fi

release_curl_args=(-fsSL -H "Accept: application/vnd.github+json")
if [[ ${#auth_args[@]} -gt 0 ]]; then
    release_curl_args+=("${auth_args[@]}")
fi
release_curl_args+=("$release_api_url")
release_json=$(curl "${release_curl_args[@]}")

release_fields=$(RELEASE_JSON="$release_json" python3 - <<'PY'
import json
import os

payload = json.loads(os.environ["RELEASE_JSON"])
for key in ("tag_name", "tarball_url", "html_url", "published_at"):
    value = payload.get(key, "")
    print(value if value is not None else "")
PY
)

release_tag=$(printf '%s\n' "$release_fields" | sed -n '1p')
tarball_url=$(printf '%s\n' "$release_fields" | sed -n '2p')
release_url=$(printf '%s\n' "$release_fields" | sed -n '3p')
released_at=$(printf '%s\n' "$release_fields" | sed -n '4p')

if [[ -z "$release_tag" || -z "$tarball_url" ]]; then
    echo "failed to resolve quickjs-ng release metadata from $release_api_url" >&2
    exit 1
fi

if [[ "$release_tag" == "${QUICKJS_NG_TAG:-}" && $force -eq 0 ]]; then
    echo "quickjs-ng already vendored at ${QUICKJS_NG_TAG}"
    exit 0
fi

tmp_dir=$(mktemp -d)
cleanup() {
    rm -rf "$tmp_dir"
}
trap cleanup EXIT

archive_path="$tmp_dir/quickjs-ng.tar.gz"
extract_dir="$tmp_dir/extracted"

mkdir -p "$extract_dir"

download_curl_args=(-fsSL)
if [[ ${#auth_args[@]} -gt 0 ]]; then
    download_curl_args+=("${auth_args[@]}")
fi
download_curl_args+=("$tarball_url" -o "$archive_path")
curl "${download_curl_args[@]}"

tar -xzf "$archive_path" -C "$extract_dir"

source_dir=$(find "$extract_dir" -mindepth 1 -maxdepth 1 -type d | head -n 1)
if [[ -z "$source_dir" ]]; then
    echo "failed to locate extracted quickjs-ng source directory" >&2
    exit 1
fi

rm -rf "$DEST_DIR"
mkdir -p "$DEST_DIR"
cp -R "$source_dir"/. "$DEST_DIR"/

cat > "$METADATA_FILE" <<EOF
QUICKJS_NG_REPO="${UPSTREAM_REPO}"
QUICKJS_NG_TAG="${release_tag}"
QUICKJS_NG_TARBALL_URL="${tarball_url}"
QUICKJS_NG_RELEASE_URL="${release_url}"
QUICKJS_NG_RELEASED_AT="${released_at}"
EOF

echo "synced quickjs-ng ${release_tag} into ${DEST_DIR}"