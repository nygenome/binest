# Proposed CRAM and CSI index support

This document describes a proposed extension to binest so the existing
user-facing commands can consume CRAM and CSI indexes in addition to the
currently supported BAI and TBI indexes.

The compatibility rule for this feature is simple: existing BAI and TBI command
behavior is the anchor. The implementation must not rename commands, remove
flags, change TSV headers, change sample ordering, or change BAI/TBI output
semantics. New formats should fit into the existing commands without requiring
users to learn a separate CRAM or CSI mode.

The relevant format sources are maintained in
[samtools/hts-specs](https://github.com/samtools/hts-specs):

- [SAM/BAM and BAI](https://samtools.github.io/hts-specs/SAMv1.pdf)
- [Tabix](https://samtools.github.io/hts-specs/tabix.pdf)
- [CSI v1](https://samtools.github.io/hts-specs/CSIv1.pdf)
- [CRAM v3.1](https://samtools.github.io/hts-specs/CRAMv3.pdf)
- [CRAM v2.1](https://samtools.github.io/hts-specs/CRAMv2.1.pdf)

## Overview

Today binest estimates index-derived density from BAI and TBI linear index
offsets. BAI and TBI both store a linear index with one virtual offset for each
16 kb reference window. binest subtracts adjacent virtual offsets to estimate
the amount of indexed data associated with each window, then normalizes those
raw density estimates by the autosomal median. The same normalized signal feeds
`size`, `chromcopy`, and `sex`.

CRAM and CSI do not both expose that same 16 kb linear index:

- CRAI indexes CRAM slices. A row points to a CRAM slice and records that
  slice's reference id, alignment start, alignment span, container byte offset,
  slice byte offset, and slice size.
- CSI is a generalized binning index. It stores bins, chunks, optional
  metadata pseudo-bins, and `loffset` values, but it does not store the BAI/TBI
  16 kb linear interval array.

The extension should therefore introduce a shared index-density window model
inside binest. Each supported index reader should produce rows shaped like:

```text
ref_id, start, end, raw_density
```

`START` remains 0-based inclusive and `END` remains 0-based exclusive in the
public TSV output. BAI and TBI continue to emit the current 16 kb windows.
CRAI and CSI may emit native index-density windows with different spans, but the
raw density value is scaled to a 16 kb-equivalent estimate so the existing
normalization, chromosome-copy, and sex-estimation code paths remain comparable
across formats.

## User experience

The same commands should accept all supported index paths:

```shell
binest size sample.bam.bai
binest size sample.vcf.gz.tbi --fai ref.fa.fai
binest size sample.cram.crai
binest size sample.bam.csi
binest size sample.cram.csi
binest size sample.vcf.gz.csi --fai ref.fa.fai
binest numreads sample.bam.csi
```

There should be no mandatory `--format` flag. binest should detect the index
kind from the suffix and validate the file magic before parsing. A recognized
extension with the wrong magic should fail as a malformed index, not silently
fall through to another parser.

The existing stdin behavior remains unchanged: command-line index arguments are
processed first, then newline-delimited index paths from stdin are processed in
order. Mixed index kinds are allowed in one invocation as long as each index has
the reference information required by its format.

The existing `--fai` and `--reference-build` flags remain the main reference
controls. The existing `--allow-bam-fai-mismatch` flag should continue to work
for compatibility. A clearer alias, `--allow-reference-mismatch`, should be
added for new documentation and examples. Both flags should set the same
validation policy.

Output headers stay exactly as they are today:

```text
CHROM START END RAW_SIZE SAMPLE
CHROM START END NORMALIZED_SIZE SAMPLE
SAMPLE CHROM COPY_ESTIMATE NORM_ESTIMATE
SAMPLE ESTIMATED_GENDER SEX_GENOTYPE NORM_X NORM_Y
SAMPLE NUM_READS
```

The `size` docs should be updated to say that BAI/TBI rows are fixed 16 kb
windows, while CRAM/CSI rows are index-density windows whose `START` and `END`
describe their actual span. Users who need the span can continue deriving it
from `END - START`; no new TSV column is needed.

Sample names should follow the existing suffix-stripping behavior and add the
new common suffixes:

- `.cram.crai`, `.crai`
- `.bam.csi`, `.cram.csi`
- `.vcf.gz.csi`, `.bed.gz.csi`

Existing suffix behavior for `.bai` and `.vcf.gz.tbi` must not change.

## Shared density model

The implementation should keep `ReadBins` stable for existing BAI/TBI tests and
callers, but route new CLI construction through a density-window reader.

The internal density row should carry:

- `RefID int`
- `Start uint64`
- `End uint64`
- `RawDensity float64`

`RawDensity` is a float because CRAM and CSI density values can be scaled from
native windows to a 16 kb-equivalent estimate. BAI and TBI values remain exact
whole-number virtual-offset differences, so their raw TSV values should keep
the same integer-looking strings they have today. For non-integer CRAM/CSI
values, format the value with Go's shortest round-trip decimal formatting.

The shared reader should apply these rules before building `Sizes`:

1. Drop rows with no reference id, missing reference name, excluded chromosome,
   non-positive span, or non-positive density.
2. Apply build-specific zero-bin masking.
3. Preserve input sample order and reference order.
4. Build the same logical `Sizes`, `ChromCopy`, and `Sex` data used today.

Normalization remains median-based:

- Autosomal rows are all emitted nonzero density rows whose chromosome is not
  `X`, `Y`, `chrX`, or `chrY`.
- The autosomal median must be positive and finite before derived values are
  written.
- `chromcopy` and `sex` use the same per-chromosome medians and rounding rules
  as the current BAI path.

## Format details

### BAI

BAI behavior stays unchanged.

The SAM/BAM specification defines BAI as a BAM index with a binning index and a
linear index. The linear index records one virtual file offset for each 16 kb
window. binest should continue using adjacent linear-index offsets:

```text
raw_density(window_n) = voffset(interval_n+1) - voffset(interval_n)
```

The output row span remains:

```text
start = n * 16384
end   = start + 16384
```

BAI pseudo-bin mapped/unmapped statistics continue to power `numreads` exactly
as they do today.

### TBI

TBI behavior stays unchanged.

The tabix specification defines a BGZF-compressed index with sequence names,
bins, chunks, and a 16 kb linear index. binest should continue reading the
linear index and using adjacent virtual offsets in the same way as BAI.

TBI still requires `--fai` because the index contains reference names but not
reference lengths. The tabix reference names should continue to be compared with
the supplied FAI before output labels are used.

### CSI v1

The hts-specs CSI document currently defines CSI v1. A CSI v1 file has magic
`CSI\1` followed by `min_shift`, `depth`, `l_aux`, `aux`, `n_ref`, per-reference
bins, and optional `n_no_coor`.

Each normal CSI bin contains:

- `bin`: the distinct bin number
- `loffset`: virtual offset of the first overlapping record
- `n_chunk`
- `chunk_beg`, `chunk_end` pairs

CSI files may also contain metadata pseudo-bins. The pseudo-bin number is:

```text
bin_limit(min_shift, depth) + 1
```

where hts-specs defines:

```text
bin_limit = ((1 << ((depth + 1) * 3)) - 1) / 7
```

Normal density rows should ignore pseudo-bins. `numreads` should use pseudo-bin
mapped/unmapped counts when present.

CSI bin spans are derived from the generalized binning geometry in hts-specs.
For a bin at level `level`, where level 0 spans the whole indexed coordinate
range and `depth` is the smallest-bin level:

```text
level_offset(level) = ((1 << (level * 3)) - 1) / 7
width(level)        = 1 << (min_shift + (depth - level) * 3)
ordinal             = bin - level_offset(level)
start               = ordinal * width(level)
end                 = start + width(level)
```

The level for a bin is the level whose interval of bin numbers contains the
bin:

```text
level_offset(level) <= bin < level_offset(level + 1)
```

Bins outside `[0, bin_limit)` are invalid for density unless they are the
metadata pseudo-bin. Unknown metadata bins should be treated as malformed input
rather than silently converted to density rows.

For each normal bin, raw CSI density is:

```text
raw_chunk_span = sum of merged chunk virtual-offset spans
raw_density    = raw_chunk_span * 16384 / (end - start)
```

Chunk virtual-offset spans use the same virtual offset conversion as the
current BAI/TBI path:

```text
virtual_offset = compressed_file_offset << 16 | uncompressed_block_offset
span           = chunk_end_virtual_offset - chunk_beg_virtual_offset
```

Chunks must be sorted and merged before summing so overlapping or adjacent
chunk ranges are counted once. A chunk with `end < begin` is malformed. A bin
with zero chunks is ignored for density.

This design emits CSI's native bin-density rows. These rows can be wider than
16 kb and can reflect the hierarchical nature of CSI bins. The density value is
scaled to a 16 kb-equivalent estimate so downstream normalization remains on the
same scale as BAI/TBI.

The optional trailing `n_no_coor` should be parsed for validation, but not
included in `numreads` unless a separate, explicitly planned behavior change
also updates BAI `numreads`. Current BAI `numreads` sums per-reference
pseudo-bin mapped counts and, with `--include-unmapped`, per-reference placed
unmapped counts.

### CRAI

CRAI is the external CRAM slice index described by the CRAM specification. It is
a gzipped, tab-delimited text file. Each row represents a slice and contains:

1. Reference sequence id
2. Alignment start
3. Alignment span
4. Absolute byte offset of the CRAM container header
5. Relative byte offset of the slice header block from the end of the container
   header
6. Slice size in bytes, including the slice header and all blocks

CRAI rows should be parsed with `compress/gzip` rather than BGZF-specific code.
All fields are decimal integers. Blank lines are ignored. Lines with the wrong
number of fields, nonnumeric values, negative slice sizes, or zero mapped spans
are malformed.

For mapped rows, convert CRAM coordinates to binest coordinates as:

```text
start = alignment_start - 1
end   = start + alignment_span
```

CRAM alignment starts are 1-based for mapped slices. binest output remains
0-based half-open.

Rows with reference id `-1` represent unmapped unplaced slices. They cannot be
assigned a chromosome coordinate, so `size`, `chromcopy`, and `sex` ignore them.
Rows with reference id `-2` should not appear in CRAI output rows for
multi-reference slices; hts-specs says multi-reference slices should have one
row per actual reference id. A CRAI row with `-2` is malformed for this
feature.

For each mapped CRAI row:

```text
raw_density = allocated_slice_size * 16384 / alignment_span
```

The `allocated_slice_size` is normally the row's slice size. Multi-reference
slices may have multiple CRAI rows for the same physical slice. To avoid
counting the full slice size once for every reference, group rows by:

```text
container_offset, slice_offset, slice_size
```

If a group has more than one mapped reference row, split the slice size evenly
across mapped rows in the group:

```text
allocated_slice_size = slice_size / mapped_row_count
```

Use floating-point division so there is no remainder policy and the total
allocated size remains equal to the original slice size.

CRAI does not contain mapped/unmapped read counts, so `numreads` should reject
CRAI with a clear unsupported-index-statistics error.

### CRAM versions

CRAM indexing is external to the CRAM file format, but the CRAM specification
describes the valid CRAM major/minor versions as 1.0, 2.0, 2.1, 3.0, and 3.1.
The CRAI table shape described above is the relevant index format for this
feature.

When a matching CRAM file is available for reference labels, binest should read
only the CRAM file definition and header container. It should not decode
alignment records and should not require reference bases for this feature.

## Reference handling

Reference handling should preserve today's safety checks and extend them to the
new formats.

BAI:

- Prefer the matching BAM header.
- If `--fai` is provided, compare BAM header names, order, and lengths with the
  FAI before using FAI labels.
- If no BAM is available, allow FAI-only behavior as today.

TBI:

- Require `--fai`.
- Compare tabix index names and order with the FAI before using FAI labels.

CRAI:

- Prefer the matching CRAM header for names, lengths, and order.
- If `--fai` is provided with a matching CRAM header, compare CRAM header names,
  order, and lengths with the FAI before using FAI labels.
- If the CRAM file is unavailable, require `--fai`.

CSI:

- For `.bam.csi`, prefer the matching BAM header.
- For `.cram.csi`, prefer the matching CRAM header.
- For tabix-style `.vcf.gz.csi` and `.bed.gz.csi`, require `--fai`.
- Do not treat CSI `aux` as a required source of truth for reference names.
  hts-specs defines `aux` as bytes, so binest should only use recognized aux
  metadata as a validation aid in a later explicit pass.

Matching sidecar paths should be derived predictably:

- `.cram.crai` -> `.cram`
- `.crai` -> remove `.crai`, then also try adding `.cram` if needed
- `.bam.csi` -> `.bam`
- `.cram.csi` -> `.cram`
- `.vcf.gz.csi` -> `.vcf.gz`
- `.bed.gz.csi` -> `.bed.gz`

Reference-build auto-detection still requires primary or sex chromosome
lengths. If no header or FAI provides those lengths, users must pass
`--reference-build b37`, `--reference-build b38`, or `--reference-build none`.

## Zero-bin masking

BAI and TBI zero-bin masking remains exactly keyed to the current 16 kb bin
number.

For CRAM and CSI rows whose spans may differ from 16 kb, compute the overlapped
16 kb tile range:

```text
first_tile = start / 16384
last_tile  = (end - 1) / 16384
```

Suppress a variable-width row only when every 16 kb tile it overlaps is masked
for that reference and reference build. If any overlapped tile is not masked,
keep the row unchanged. Do not subtract masked subregions from the density value
because that would create partial-window semantics that do not exist in the
underlying index.

If `--reference-build none` is selected, no zero-bin masking is applied.

## Error behavior

Errors should be direct and actionable:

- Unknown extension: keep the existing unsupported-index error shape.
- Recognized extension with wrong magic: return malformed index with the path
  and expected magic.
- Unsupported CSI version: report unsupported CSI version and include the path.
- CSI bin outside the valid range: report malformed CSI bin.
- CSI chunk end before chunk begin: report malformed CSI chunk.
- CRAI not gzip-compressed: report malformed CRAI compression.
- CRAI wrong column count or nonnumeric field: report malformed CRAI row and
  include the 1-based line number.
- CRAI mapped row with `alignment_start <= 0` or `alignment_span <= 0`: report
  malformed CRAI coordinates.
- CRAI row with reference id `-2`: report malformed CRAI multi-reference row.
- Missing BAM/CRAM sidecar: fall back to `--fai` when that format allows it;
  otherwise report the missing reference source.
- Missing CSI pseudo-bin stats for `numreads`: report that index statistics are
  unavailable for that CSI file.
- CRAI passed to `numreads`: report that CRAI does not contain mapped/unmapped
  read statistics.

Batch processing should continue to match current behavior: write the command
header once, process each input path independently, collect per-index errors,
and return a nonzero exit status if any input failed.

## Implementation sequence

This design should be implemented in small reviewable patches:

1. Add this design document.
2. Add index kind detection and suffix stripping for CRAI and CSI.
3. Add the shared density-window model while preserving `ReadBins` for BAI/TBI.
4. Route BAI/TBI through the density model and prove their goldens do not
   change.
5. Add CSI v1 parsing, density conversion, pseudo-bin stats, and tests.
6. Add CRAI parsing, density conversion, multi-reference slice splitting, and
   tests.
7. Add CRAM header reference lookup and extend reference validation.
8. Add CLI help and README updates for new examples and
   `--allow-reference-mismatch`.
9. Add checksum-verified real fixtures and compact goldens for `.crai` and
   `.csi`.

Do not combine dependency, toolchain, or CI updates with the behavior
implementation unless they are required to parse the new formats.

## Test plan

For this doc-only change, no Go tests are required.

For the implementation patches, run:

```shell
go fmt ./...
go test ./...
make test-real
make check
```

Add focused synthetic tests for:

- index kind detection and sample-name suffix stripping
- CSI magic/version validation
- CSI bin-to-span conversion at multiple `min_shift` and `depth` values
- CSI pseudo-bin detection and `numreads` stats
- CSI chunk sorting, merging, and malformed chunk rejection
- CRAI gzip parsing and row validation
- CRAI 1-based to 0-based coordinate conversion
- CRAI unmapped row skipping
- CRAI multi-reference slice size splitting
- variable-width zero-bin masking
- missing BAM/CRAM sidecar fallback to FAI
- CLI regression behavior for headers, stdin, and batch errors

Extend the GIAB real-data manifest with public, checksum-verified `.crai` and
`.csi` index fixtures only. Do not commit BAM, BAI, TBI, VCF, BED, CRAM, CRAI,
fixture caches, built binaries, or generated full TSV outputs. Compact goldens
should continue to store hashes, row counts, headers, first/last rows, and
sentinel rows.
