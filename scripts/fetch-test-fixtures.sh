#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/fetch-test-fixtures.sh [--check] [--force] [--print-cache]

Downloads and verifies public GIAB test fixtures from testdata/giab/manifest.tsv.

Options:
  --check        Verify cached files only; do not download.
  --force        Remove cached files and download them again.
  --print-cache  Print the cache directory and exit.
USAGE
}

mode="fetch"
force=0

while (($#)); do
  case "$1" in
    --check)
      mode="check"
      ;;
    --force)
      force=1
      ;;
    --print-cache)
      mode="print-cache"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
manifest="${repo_root}/testdata/giab/manifest.tsv"
cache_dir="${BINEST_FIXTURE_CACHE:-${repo_root}/.cache/binest-fixtures/giab}"

if [[ "${mode}" == "print-cache" ]]; then
  printf '%s\n' "${cache_dir}"
  exit 0
fi

if [[ ! -f "${manifest}" ]]; then
  echo "missing fixture manifest: ${manifest}" >&2
  exit 1
fi

sha256_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${path}" | awk '{print $1}'
  else
    shasum -a 256 "${path}" | awk '{print $1}'
  fi
}

file_size() {
  wc -c < "$1" | tr -d '[:space:]'
}

verify_file() {
  local path="$1"
  local want_bytes="$2"
  local want_sha="$3"

  [[ -f "${path}" ]] || return 1

  local got_bytes
  got_bytes="$(file_size "${path}")"
  if [[ "${got_bytes}" != "${want_bytes}" ]]; then
    echo "size mismatch for ${path}: got ${got_bytes}, want ${want_bytes}" >&2
    return 1
  fi

  local got_sha
  got_sha="$(sha256_file "${path}")"
  if [[ "${got_sha}" != "${want_sha}" ]]; then
    echo "sha256 mismatch for ${path}: got ${got_sha}, want ${want_sha}" >&2
    return 1
  fi
}

mkdir -p "${cache_dir}"

failures=0

while IFS=$'\t' read -r id sample role kind filename bytes sha256 url source_page purpose; do
  if [[ -z "${id}" || "${id}" == "id" ]]; then
    continue
  fi

  dest="${cache_dir}/${filename}"
  part="${dest}.part"

  if [[ "${force}" == "1" ]]; then
    rm -f "${dest}" "${part}"
  fi

  if verify_file "${dest}" "${bytes}" "${sha256}"; then
    echo "verified ${id}: ${dest}"
    continue
  fi

  if [[ "${mode}" == "check" ]]; then
    echo "missing or invalid fixture ${id}: ${dest}" >&2
    failures=$((failures + 1))
    continue
  fi

  rm -f "${dest}"
  echo "downloading ${id}: ${url}"
  curl --fail --location --retry 5 --retry-all-errors --continue-at - --output "${part}" "${url}"

	if ! verify_file "${part}" "${bytes}" "${sha256}"; then
		echo "downloaded fixture failed verification: ${part}" >&2
		rm -f "${part}"
		failures=$((failures + 1))
		continue
	fi

  mv "${part}" "${dest}"
  echo "verified ${id}: ${dest}"
done < "${manifest}"

if ((failures > 0)); then
  echo "${failures} fixture(s) failed verification" >&2
  exit 1
fi
