# AGENTS.md

## Project overview

`binest` estimates genomic data density and derived signals from BAI and TBI
index files. The core user-facing commands are `size`, `chromcopy`, `sex`, and
`numreads`.

Correct scientific behavior, stable TSV output, reproducible fixtures, and
small reviewable changes matter more than broad refactors. Treat the current
GIAB-backed tests as the main protection against accidental behavior changes.

## Development workflow

- Keep changes focused and commit in small reviewable units.
- Prefer repo patterns over new abstractions.
- Run `go fmt ./...` after editing Go files.
- Run `go test ./...` for any code change.
- Run `make test-real` when changing index decoding, normalization, reference
  build detection, sex/copy estimation, GIAB fixtures, or compact goldens.
- Run `make check` before finishing a pass.
- Do not push branches unless explicitly asked.

## Behavior compatibility

- Do not change CLI commands, flags, TSV headers, sample ordering, or output
  semantics unless the change is explicitly planned.
- Before fixing scientific behavior, add or update tests that prove both the
  old behavior and the intended corrected behavior.
- Keep `numreads` as a control path when changing `size`, `chromcopy`, or
  `sex`.
- If normalized output changes, update compact GIAB goldens and explain why the
  change is expected.

## Test and fixture expectations

- Use focused unit and synthetic tests for edge cases that real fixtures cannot
  isolate.
- Use GIAB-backed real-data tests for index parsing, real output shape, and
  behavior changes that affect TSV values.
- Keep committed real-data goldens compact: hashes, row counts, headers,
  first/last rows, and sentinel rows.
- Do not commit BAM, BAI, TBI, VCF, BED, CRAM, CRAI, fixture caches, built
  binaries, or generated full TSV outputs.
- Keep GIAB fixture provenance direct and checksum verified.
- Only refresh compact GIAB goldens intentionally, using the documented refresh
  path.

## Go expectations

- Follow `gofmt` and idiomatic Go naming.
- Handle errors directly. Do not ignore, swallow, or overwrite errors unless
  there is a clear reason.
- Wrap errors when added context helps callers understand the failing path.
- Keep resource cleanup explicit and checked, especially around file handles,
  BGZF readers, BAM readers, and output writers.
- Avoid broad interfaces and new abstractions unless they remove real
  complexity.
- Keep tests readable, table-driven where useful, and make failure messages say
  what input was used and what was expected.

## Dependency and security expectations

- Use Go module tooling for dependency changes.
- Keep `go.mod` and `go.sum` tidy.
- Do not bundle dependency, toolchain, or CI updates into behavior-only PRs
  unless they are required for the behavior change.
- Keep `govulncheck`, Dependabot, and CI findings actionable. Do not silence
  them without documenting why.
- Be careful with `unsafe` and reflection around Biogo index internals. Changes
  in this area need focused tests and real GIAB coverage.

## CI and repository hygiene

- Keep GitHub Actions permissions at the minimum needed for the workflow.
- Preserve idempotent fixture download and cache behavior.
- Preserve `git-size-check` protections against tracked binaries and large
  genomics artifacts.
- Do not use `pull_request_target` or write-capable workflow permissions unless
  a separate security review justifies it.

## Review guidelines

Flag these as blocking issues:

- Behavior changes without tests or golden updates.
- CLI flags, TSV headers, sample ordering, or output semantics changing without
  being explicitly planned.
- GIAB fixture URLs, byte sizes, checksums, or provenance changing without a
  clear reason.
- Large biological files, built binaries, fixture caches, or generated full TSV
  outputs being tracked by Git.
- Dependency, toolchain, or CI changes bundled into behavior-only PRs without
  explanation.
- Ignored errors, swallowed write failures, unsafe cleanup, or resource leaks in
  index parsing and output paths.
- GitHub Actions permission expansion or workflow changes that weaken
  reproducibility or security.
- Unsafe/reflection changes around Biogo index internals without real GIAB
  coverage.

Treat style-only comments as nonblocking unless they hide a correctness,
maintainability, or reproducibility problem.
