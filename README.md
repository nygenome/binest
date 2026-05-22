# binest

##### Description

binest calculates chromcopy, sex and normalized sizes per 16kb chunk in the genome
from BAM and tabix indexes.

The `size`, `chromcopy`, and `sex` commands use index-derived density proxies.
Raw values are estimated from BGZF virtual-offset movement in the index. They are
useful for relative density and normalization, but they are not exact read counts
or exact byte counts for a genomic window.

In order to map the chunk values back to their genomic co-ordinates,
binest tries to read the BAM header for the corresponding BAM file.
If the BAM file doesn't exist, the reference FAI index must be provided. When
both a BAM header and `--fai` are available, binest validates that the references
have the same order, names, and lengths before using the FAI labels. Use
`--allow-bam-fai-mismatch` only when you intentionally want warnings instead of
errors and understand that coordinates or chromosome labels may be wrong.

The `--reference-build` flag on `size`, `chromcopy`, and `sex` controls the
build-specific zero-bin mask. The default `auto` mode detects b37 or b38 from
primary and sex chromosome lengths. If the build cannot be detected, pass
`--reference-build b37`, `--reference-build b38`, or `--reference-build none`.
The `none` value disables zero-bin masking.

Note: Any TABIX indexed file can be used with binest to get an idea of data density across the genome.
The reference FAI index must always be provided when working with TABIX indexes.
Compact FAI files are supported for BAI-only workflows when no BAM header is
available, but compact files cannot validate references that are absent from the
FAI and may not provide enough evidence for `--reference-build auto`.

The `numreads` command is independent of FAI files, reference-build detection,
and zero-bin masking. It reads mapped and optionally unmapped read counts from
the BAM index statistics path.


##### Installation

```shell
go install github.com/nygenome/binest/cmd/binest@latest
```


##### Example usage

The basic usage for all binest commands are the same.
Examples below show usage for the size command.

```shell
# Scenario 1 - BAM and BAI for sample present
binest size [PATH_TO_BAI_FILE]
binest size [PATH_TO_BAI_FILE1] [PATH_TO_BAI_FILE2]...
ls {PROJECT}/{SAMPLE}_*/*.bai | binest size

# Scenario 2 - BAI only present. BAM not present.
binest size --fai [REFERENCE.fasta.fai] [PATH_TO_BAI_FILE]
binest size -f [REFERENCE.fasta.fai] [PATH_TO_BAI_FILE1] [PATH_TO_BAI_FILE2]...
ls {PROJECT}/{SAMPLE}_*/*.bai | binest size -f [REFERENCE.fasta.fai]

# Disable zero-bin masking when the reference build is intentionally unknown.
binest size --fai [REFERENCE.fasta.fai] --reference-build none [PATH_TO_BAI_FILE]

# Scenario 3 - TBI index.
binest size --fai [REFERENCE.fasta.fai] [PATH_TO_TBI_FILE]
binest size -f [REFERENCE.fasta.fai] [PATH_TO_TBI_FILE1] [PATH_TO_TBI_FILE2]...
ls {PROJECT}/{SAMPLE}_*/*.tbi | binest size -f [REFERENCE.fasta.fai]

## Additional parameters can be seen by running
binest -h
```
