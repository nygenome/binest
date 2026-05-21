# GIAB test fixtures

`manifest.tsv` is the source of truth for the public GIAB index fixtures used by
the real-data behavior tests. The files are downloaded from NCBI GIAB FTP over
HTTPS and are not committed to this repository.

## Cache policy

Run `make fixtures-fetch` to download the fixtures into:

```text
.cache/binest-fixtures/giab
```

Set `BINEST_FIXTURE_CACHE` to override the cache location. The fetch script is
idempotent, writes partial downloads to `*.part`, verifies byte size and SHA256,
then atomically moves verified files into place.

## Provenance

The BAI fixtures are from GIAB high-coverage GRCh38 BAM indexes for HG001 and
the Ashkenazim Trio:

- HG001/NA12878 300x HiSeq novoalign BAM index.
- HG002/NA24385 son 2x250 novoalign BAM index.
- HG003/NA24149 father 2x250 novoalign BAM index.
- HG004/NA24143 mother 2x250 novoalign BAM index.

The TBI fixtures are from the GIAB NIST v4.2.1 GRCh38 benchmark VCF indexes for
the same samples.

Each row in `manifest.tsv` records the direct file URL, source directory, byte
size observed from the NCBI HTTPS response, and SHA256 computed from the
downloaded bytes.

## Why fixtures are not committed

The approved GIAB index fixtures are small for genomics data, but together they
are about 45 MiB. This repository previously carried large generated artifacts
in Git history, so the test data policy is intentionally conservative: commit
provenance, checksums, expected behavior metadata, and scripts; keep downloaded
biological data in an ignored cache.
