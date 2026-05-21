#!/usr/bin/env bash
set -euo pipefail

max_bytes="${BINEST_MAX_TRACKED_BYTES:-1048576}"
failures=0

while IFS= read -r path; do
  case "${path}" in
    bin/*|testdata/giab/cache/*|.cache/binest-fixtures/*|testdata/*/generated/*|\
*.bam|*.bai|*.cram|*.crai|*.vcf.gz|*.vcf.gz.tbi|*.bed.gz|*.bed.gz.tbi)
      echo "forbidden tracked fixture/build artifact: ${path}" >&2
      failures=$((failures + 1))
      continue
      ;;
  esac

  if [[ -f "${path}" ]]; then
    size="$(wc -c < "${path}" | tr -d '[:space:]')"
    if ((size > max_bytes)); then
      echo "tracked file exceeds ${max_bytes} bytes: ${path} (${size} bytes)" >&2
      failures=$((failures + 1))
    fi
  fi
done < <(git ls-files)

if ((failures > 0)); then
  exit 1
fi
