#!/usr/bin/env bash
set -euo pipefail

max_bytes="${BINEST_MAX_TRACKED_BYTES:-1048576}"
failures=0

forbidden_path() {
  case "$1" in
    bin/*|testdata/giab/cache/*|.cache/binest-fixtures/*|testdata/*/generated/*|\
*.bam|*.bai|*.cram|*.crai|*.vcf.gz|*.vcf.gz.tbi|*.bed.gz|*.bed.gz.tbi)
      return 0
      ;;
  esac
  return 1
}

record_failure() {
  echo "$1" >&2
  failures=$((failures + 1))
}

while IFS= read -r path; do
  if forbidden_path "${path}"; then
    record_failure "forbidden tracked fixture/build artifact: ${path}"
    continue
  fi

  if [[ -f "${path}" ]]; then
    size="$(wc -c < "${path}" | tr -d '[:space:]')"
    if ((size > max_bytes)); then
      record_failure "tracked file exceeds ${max_bytes} bytes: ${path} (${size} bytes)"
    fi
  fi
done < <(git ls-files)

history_base="${BINEST_GIT_SIZE_BASE:-}"
if [[ -z "${history_base}" && -n "${GITHUB_BASE_REF:-}" ]]; then
  history_base="origin/${GITHUB_BASE_REF}"
fi

if [[ -n "${history_base}" ]]; then
  if ! git rev-parse --verify --quiet "${history_base}^{commit}" >/dev/null; then
    echo "could not resolve git-size history base: ${history_base}" >&2
    exit 1
  fi

  while IFS= read -r -d '' path; do
    if forbidden_path "${path}"; then
      record_failure "forbidden fixture/build artifact path in history since ${history_base}: ${path}"
    fi
  done < <(git log -z --format= --name-only --diff-filter=ACMR "${history_base}..HEAD")

  while IFS= read -r object_path; do
    object="${object_path%% *}"
    path=""
    if [[ "${object_path}" == *" "* ]]; then
      path="${object_path#* }"
    fi
    if [[ -z "${path}" ]]; then
      continue
    fi

    object_type="$(git cat-file -t "${object}")"
    if [[ "${object_type}" != "blob" ]]; then
      continue
    fi

    if forbidden_path "${path}"; then
      record_failure "forbidden fixture/build artifact in history since ${history_base}: ${path}"
      continue
    fi

    size="$(git cat-file -s "${object}")"
    if ((size > max_bytes)); then
      record_failure "historical blob exceeds ${max_bytes} bytes since ${history_base}: ${path} (${size} bytes)"
    fi
  done < <(git rev-list --objects "${history_base}..HEAD")
fi

if ((failures > 0)); then
  exit 1
fi
