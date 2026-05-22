# CRAM and CSI support design exploration

This document explores whether and how binest can safely support CRAM/CRAI
and CSI indexes in addition to the existing BAI and TBI paths.

This is intentionally not an implementation plan. It does not commit the
project to shipping CRAM or CSI support until the scientific and user-experience
questions below have satisfactory answers. It also avoids prescribing mechanics
where the index formats do not obviously expose the same signal binest currently
uses.

The project anchor is compatibility with today's useful behavior: binest gives
fast, approximate genomic data-density and derived QC estimates from small index
files. That value is strongest when users can inspect `.bai` or `.tbi` files
without downloading the much larger BAM, VCF, BED, or CRAM payloads. Any CRAM or
CSI design that requires source data access must therefore justify the added
cost against the improvement in correctness.

## Primary sources

The source of truth for format behavior is
[samtools/hts-specs](https://github.com/samtools/hts-specs), which identifies
SAM/BAM/BAI, CRAM, tabix, and CSI as the maintained canonical specifications.
The design space below is grounded in these files:

- [SAMv1.tex](https://github.com/samtools/hts-specs/blob/master/SAMv1.tex)
  for SAM, BAM, BGZF virtual offsets, and BAI.
- [tabix.tex](https://github.com/samtools/hts-specs/blob/master/tabix.tex)
  for TBI names, tabix coordinate metadata, bins, chunks, and 16 kb linear
  intervals.
- [CSIv1.tex](https://github.com/samtools/hts-specs/blob/master/CSIv1.tex)
  for CSI v1 fields, generalized binning, chunks, `loffset`, pseudo-bins, aux
  data, and optional trailing `n_no_coor`.
- [CRAMv3.tex](https://github.com/samtools/hts-specs/blob/master/CRAMv3.tex)
  for CRAM objectives, file/container/slice structure, reference compression,
  the RI data series, and CRAI rows.
- [CRAMv2.1.tex](https://github.com/samtools/hts-specs/blob/master/CRAMv2.1.tex)
  for the older CRAM version family.

hts-specs currently defines CSI v1. Future CSI versions should be treated as a
separate compatibility question, not silently accepted as if they were v1.

CRAM version coverage is also a separate feasibility axis. The CRAM v3
specification lists CRAM 1.0, 2.0, 2.1, 3.0, and 3.1 as valid historical
versions. The CRAI table shape is conceptually stable across these versions,
but reading CRAM headers, containers, or data series may not be equally simple
or fixture-backed for all of them.

## Purpose and non-goals

The purpose of this document is to explore design choices for adding new index
families without damaging the existing BAI/TBI behavior. The questions are:

- Can CSI be mapped onto binest's current density model without invalidating
  `size`, `chromcopy`, or `sex`?
- Can CRAI/CRAM provide a scientifically defensible density signal, or does
  CRAM compression make the signal too format-dependent for derived calls?
- Can reference naming and length validation remain robust while preserving the
  index-only workflow that makes binest convenient?
- Can users pass `.bai`, `.tbi`, `.crai`, and `.csi` paths through familiar
  commands without a confusing or fragile UX?

Non-goals for this document:

- It is not an implementation sequence.
- It is not a test plan.
- It does not choose a final CRAM density estimator.
- It does not choose a final CSI density attribution rule.
- It does not choose a final multi-reference CRAI apportionment rule.
- It does not allow existing BAI/TBI output, TSV headers, sample ordering,
  command names, or required flags to change as a side effect of exploration.

Any future implementation should keep existing behavior as the compatibility
anchor. In particular, BAI/TBI outputs should remain byte-for-byte stable unless
a separate, explicit behavior change is approved.

## How binest works today

### The current signal

In its current BAI and TBI modes, binest is not measuring exact coverage. It is
measuring movement through an indexed, BGZF-compressed file and using that
movement as a proxy for data density.

BAI and TBI both expose a linear index at 16 kb resolution:

- For each reference, the linear index has one virtual offset per 16 kb genomic
  interval.
- A BGZF virtual offset combines the compressed file block offset and the
  offset within the decompressed BGZF block.
- binest compares adjacent linear-index offsets and treats their difference as
  the raw density estimate for that 16 kb interval.

This works tolerably for binest because the observations are uniform. A raw
density value for one autosomal 16 kb tile is directly comparable to another
raw density value from the same index, subject to known compression and record
layout caveats. The signal is still approximate: BGZF movement depends on
compression behavior, record content, local alignment complexity, quality/tag
content, and how records are packed into blocks. It is not a read-count,
base-depth, or byte-for-byte coverage oracle.

The important property is not that the proxy is perfect. The important property
is that BAI/TBI provide a consistent observation grid. Because every emitted
row represents the same 16 kb genomic span, the existing median logic is
implicitly close to a base-pair-weighted median. A row-weighted median over
equal-width rows is a reasonable approximation for normalization.

### Derived commands

The current commands build on that 16 kb proxy:

- `size --raw` reports the raw index-density estimate for each nonzero,
  non-masked 16 kb tile.
- `size` normalizes each raw tile by the autosomal median raw density.
- `chromcopy` computes chromosome-level medians from the same normalized signal
  and converts them into copy estimates.
- `sex` uses the normalized X and Y signals from the same median-based model.
- `numreads` is separate: it reads BAI mapped/unmapped statistics rather than
  deriving read counts from the density model.

TBI deserves a small semantic caveat. A tabix index can describe VCF, BED, or
generic genomic text records rather than aligned reads. For tabix-backed data,
the density signal is still "indexed record density through a BGZF file," not
sequencing depth in the BAM sense. The stable 16 kb grid still makes the signal
internally comparable enough for the current output model.

### Reference handling and UX expectations

Existing users rely on several behavior guarantees:

- `size`, `chromcopy`, `sex`, and `numreads` keep stable command names.
- TSV headers and sample ordering are stable.
- Command-line index arguments and stdin batches are processed in order.
- BAI can use a matching BAM header when available, or a supplied FAI fallback.
- TBI requires `--fai` because the index has names but not lengths.
- When both an index-derived reference source and an FAI are available, binest
  validates reference order, names, and lengths where possible before labeling
  output.
- Build auto-detection and zero-bin masking depend on trustworthy primary and
  sex chromosome lengths.

These expectations should not get worse for BAI/TBI. For new formats, the
design question is how close we can get without making users supply large
source files in the common case.

## What the new formats actually contain

### CSI

CSI is a generalized binning index. It was designed as a successor to BAI and
can represent coordinate ranges beyond BAI's limits and non-BAI binning
parameters.

The CSI v1 header includes:

- magic `CSI\1`
- `min_shift`, the number of bits in the smallest bin size
- `depth`, the depth of the binning hierarchy
- `l_aux`, the length of auxiliary data
- `aux`, opaque auxiliary bytes from the CSI point of view
- `n_ref`, the number of indexed references

Each reference then has distinct bins. Each normal bin has:

- a bin number
- `loffset`, a virtual offset associated with the first overlapping record
- zero or more chunks, each with begin and end virtual offsets

CSI also defines pseudo-bins, whose bin number is derived from
`bin_limit(min_shift, depth) + 1`. For alignment indexes, these may contain
BAI-like mapped and unmapped statistics. The file can also end with a single
optional trailing `n_no_coor` value.

The important difference from BAI/TBI is that CSI does not expose the same
fixed 16 kb linear interval array as the direct data model. Its units are bins
in a configurable hierarchy plus chunks and offsets. With the common
`min_shift=14, depth=5` geometry, the smallest bins are 16 kb, but CSI permits
other geometries and can contain records in coarser bins when alignments span
larger intervals.

### Tabix-style CSI

CSI can be used for tabix-style genomic text files as well as alignment files.
In that setting, the auxiliary data can carry tabix-like metadata: file format,
coordinate columns, comment/skip metadata, and concatenated reference names.

That matters for binest because current TBI handling validates tabix reference
names against the supplied FAI before using FAI lengths and names for output.
If tabix-style CSI support skips aux parsing, it risks silently mislabeling
references when the user supplies the wrong FAI. Matching the current TBI safety
model likely requires treating tabix-style aux metadata as a validation source,
not as optional decoration.

### CRAI

CRAI is a gzipped tab-delimited CRAM index table. Each row describes a CRAM
slice, not a 16 kb genomic observation. The CRAM v3 specification describes six
columns:

- reference sequence id
- alignment start
- alignment span
- absolute byte offset of the container header
- relative byte offset of the slice header block from the end of the container
  header
- slice size in bytes, including the slice header and all blocks

Each line represents a slice. Multi-reference slices may have multiple CRAI
rows for the same slice, one for each actual reference contained in the slice.
The CRAM spec states that the actual reference ID comes from the RI data series
in this case, not from the multi-reference sentinel value used in slice or
container headers.

The crucial limitation is that CRAI does not store per-read coverage or per-tile
counts. It stores where slices live and how large they are.

### CRAM

CRAM is not just BAM with a different index. It is a reference-compressed,
containerized alignment format. The CRAM v3 spec says CRAM was designed for
better lossless compression than BAM, BAM compatibility, smooth transition from
BAM, and support for controlled loss of BAM data.

Those goals are directly relevant to binest's density model. CRAM bytes are a
product of compression decisions, reference differencing, quality-score and tag
entropy, codec choices, slice overhead, and optional lossy preservation
policies. Byte size can vary for reasons that are not proportional to read
depth.

CRAM files have a file definition, a CRAM header container with SAM header text,
data containers, and slices. Single-reference slices can expose a reference id,
alignment start, alignment span, and number of records in headers. Multiple
reference slices use the RI data series to identify the reference id for each
record. Reading RI requires going beyond the CRAI table.

This creates the central CRAM question: how much of the CRAM payload must
binest inspect before the signal is defensible enough for `chromcopy` and
`sex`?

## Core modeling problem

BAI/TBI expose fixed 16 kb observations. CSI and CRAI do not necessarily do
that.

That mismatch is not a cosmetic parser issue. It affects the statistics at the
heart of binest:

- A row-weighted median is valid enough only when rows have the same genomic
  span. If a 16 kb BAI tile and a 5 Mb CRAM slice row each count as one
  observation, normalization depends on index geometry rather than genomic
  signal.
- The zero-bin mask is keyed to 16 kb tiles. A wide native row that overlaps a
  small masked region cannot be masked with the same semantics unless the model
  defines how that row maps onto tiles.
- `chromcopy` and `sex` depend on medians of comparable observations. Mixing
  native rows of different widths or different density meanings can silently
  change biological interpretation.
- Output order must remain deterministic even if CSI bins or CRAI rows are not
  stored in genomic order by every writer.
- `numreads` must not report generic tabix record counts under a `NUM_READS`
  header. Alignment-index statistics and generic record-index statistics are
  different concepts.

CRAM adds an additional risk beyond variable row width. CRAM slice bytes are
not a clean density signal:

- Reference-matching sequence compresses differently from mismatching sequence.
- Quality-score distributions can dominate compressed size.
- Tags and auxiliary fields vary by pipeline and sample.
- CRAM can use multiple codecs across data series and blocks.
- Slice header and block overhead are included in CRAI slice size.
- Alignment span is the range covered by the slice, not the exact read
  footprint within that range.
- Multi-reference slices interleave records from multiple references, while
  CRAI rows alone do not provide per-reference byte ownership.

The design task is therefore not "parse CSI and CRAI." The design task is
"derive a comparable density signal from index units that were not all designed
to mean the same thing."

## Exploration paths

The paths below are not mutually exclusive. A viable design may combine them by
format or by command.

### Path A: fixed 16 kb projection

Project every supported format onto the current 16 kb tile grid before
normalization or derived calls.

In this model, BAI and TBI remain exactly as they are. CSI and CRAI would be
interpreted into a per-reference sequence of 16 kb tile-density observations
before `size`, `chromcopy`, or `sex` consumes them. The public output could then
continue to represent fixed 16 kb windows for all derived behavior, or it could
separately expose native rows only in a carefully documented raw exploration
mode. That output choice is part of the design question.

Benefits:

- It preserves the core equal-width observation assumption.
- It avoids row-weighted median bias by construction.
- It keeps the existing zero-bin mask model coherent.
- It aligns with the current `Bins`/`Sizes` shape, which assumes per-reference
  tile arrays and emits `END = START + 16384`.
- It makes BAI/TBI compatibility easier to protect because the downstream model
  remains familiar.

Open issues:

- CSI still needs a defensible way to attribute bin/chunk/offset information
  onto 16 kb tiles, especially for records in higher-level bins.
- CRAI still needs a defensible way to distribute slice-level signal across
  all overlapped tiles.
- CRAM slice-size signal may remain biased even after projection because byte
  size is not the same thing as read count or base depth.
- Projection can create false precision if a large CRAM slice is spread across
  many tiles even though the index does not actually observe per-tile density.
- If native row spans are hidden from users, the output may look more precise
  than the input format supports.

Path A is attractive because it makes the current normalization math sane
again. It does not by itself prove that the upstream CSI or CRAM signal is
scientifically valid.

### Path B: weighted variable-width model

Keep native CSI/CRAI spans and change normalization to account for genomic
width, for example through base-pair-weighted medians or an equivalent
interval-aware statistic.

Benefits:

- It preserves the native shape of the new index formats.
- It avoids pretending that coarse rows are fine-grained observations.
- It can make raw output more transparent because `START` and `END` describe
  the actual index-derived row.

Open issues:

- It is more invasive than Path A because current result structures and output
  formatting assume 16 kb rows.
- Weighted medians would need clear semantics around zero rows, excluded
  chromosomes, masked tiles, and partially masked intervals.
- The downstream behavior of `chromcopy` and `sex` would become harder to
  compare to existing BAI/TBI results.
- It still does not solve the CRAM byte-signal problem.
- It may change user expectations if output rows are no longer visually
  comparable across formats.

Path B is conceptually honest about native index geometry, but it asks more of
the rest of the tool and still leaves the hardest CRAM question unanswered.

### Path C: scope by command

Support only the commands whose scientific meaning is defensible for each
format, instead of forcing every index kind through every output.

Examples of this path could include:

- Allowing exploratory raw density for a format before enabling normalized
  `size`.
- Supporting BAM-CSI where the signal matches the current alignment-index
  assumptions more closely, while treating tabix-CSI and CRAI differently.
- Rejecting CRAM-derived `chromcopy` and `sex` until a calibrated CRAM signal is
  available.
- Keeping `numreads` limited to true read-count statistics rather than generic
  record counts.

Benefits:

- It avoids presenting unsupported biological interpretations as if they were
  equivalent to BAI.
- It lets safer parts of CSI support move independently from riskier CRAM
  derived calls.
- It gives error messages a principled basis: "this format does not contain
  that signal" rather than "parser not implemented."

Open issues:

- The desired user-facing goal is broad support through the same commands, so
  this is a fallback posture rather than the preferred outcome.
- Partial support can feel inconsistent if the support matrix is hard to
  predict.
- It may still require a raw density definition that is scientifically weak for
  CRAM unless raw output is clearly marked as exploratory.

Path C is most useful as a safety valve. It should remain available if CRAM or
some CSI use cases cannot be made defensible for derived outputs.

### Path D: require source data for a stronger CRAM signal

Use more than the CRAI table when CRAM index-only signal is not strong enough.
This could range from light CRAM access to heavier decoding:

- Read only the CRAM file definition and CRAM header container to obtain
  reference names and lengths.
- Read container and slice headers to obtain record counts, spans, and
  container-level metadata.
- Read the RI data series for multi-reference slices to attribute records to
  references.
- Decode more record-level data if per-tile read placement is required.

Benefits:

- It can produce a count-like or placement-like signal that is closer to what
  `chromcopy` and `sex` need.
- It can solve multi-reference slice attribution in a way CRAI alone cannot.
- It can improve reference validation by using the CRAM SAM header directly.

Open issues:

- It conflicts with the index-only QC value proposition.
- It may require access to very large CRAM files that users were hoping to
  avoid downloading.
- It expands the CRAM version and codec surface area.
- "Read just enough CRAM" is a subtle boundary: CRAM headers are small, but RI
  and record placement live in data blocks and may require codec handling.
- The more source data binest reads, the less distinct it becomes from a
  coverage or alignment-inspection tool.

Path D may be necessary for scientifically confident CRAM-derived copy/sex
signals. It should be considered against the cost of losing the lightweight
index-only workflow.

### Path E: calibrated hybrid support

Use different evidence levels by format and command, with explicit calibration
as the gate for derived calls.

For example, BAM-CSI may be viable through an index-only tile projection,
tabix-CSI may be viable for record-density `size` but not `numreads`, and CRAI
may require either strong caveats or CRAM-assisted read-count signal before
`chromcopy` and `sex` are enabled.

Benefits:

- It allows each format to be evaluated on the signal it actually contains.
- It avoids treating CRAM slice bytes and BAM virtual-offset movement as
  automatically equivalent.
- It gives room for future improvement without overcommitting the first design.

Open issues:

- The UX needs to explain capability differences without making the tool feel
  unpredictable.
- Calibration must be strong enough to catch biological failure modes, not just
  parser correctness.
- Format-specific caveats can accumulate into documentation and support burden.

This path is probably how the exploration will converge if no single model
works cleanly for all formats.

## Format-specific exploration

### CSI

CSI raises several separate design questions.

#### Hierarchical bins

CSI bins are hierarchical. A genomic position is contained in a bin at every
level, but each indexed record is assigned to a particular bin according to its
span. Short-read alignments with common CSI parameters often land in the
smallest bins; long reads or large-spanning records can land in coarser bins.

Possible interpretations:

- Emit native rows for every non-empty bin. This is transparent but creates
  overlapping genomic rows across levels. A row-weighted median over those rows
  is not valid.
- Use only smallest-level bins. This preserves fixed-bin intuition when data
  are short-read-like, but it can drop or underrepresent records assigned to
  higher-level bins.
- Project all bin levels onto 16 kb tiles. This preserves long-spanning records
  but requires a rule for distributing a coarse bin's signal across its covered
  tiles.
- Treat higher-level bins as a reason to downgrade or reject derived calls for
  that file. This is safer but less capable, especially for long-read data.

The right answer may differ for BAM-CSI, CRAM-CSI, and tabix-style CSI.

#### Density from `loffset` or chunks

CSI exposes both `loffset` and chunk ranges. Neither is automatically a per-tile
density estimate.

Chunk-span interpretation:

- Uses the virtual-offset span of chunks attached to a bin.
- Has a direct analogy to "how much file range is associated with this bin."
- Can be biased by chunk coalescing and by records from nearby bins sharing
  adjacent byte ranges.
- Needs a rule for overlapping or adjacent chunks.

`loffset`-difference interpretation:

- Is closer in spirit to the BAI/TBI linear-index offset-difference model.
- Avoids some chunk coalescing attribution problems.
- Requires a coherent ordering of bins or projected tiles.
- May be undefined or weak when bins are sparse, missing, or not present at a
  uniform level.

Source-data interpretation:

- Reads the indexed BAM/CRAM/tabix payload to derive stronger per-region counts.
- Can be more accurate.
- Weakens or loses the index-only workflow.

The design should not assume that chunk spans are correct merely because they
are easy to parse. It also should not assume that `loffset` differences solve
every hierarchy and sparsity problem without evidence.

#### Pseudo-bins and `numreads`

CSI pseudo-bins can carry mapped and unmapped statistics similar to BAI. For
alignment indexes, those statistics may support `numreads` in a way that fits
the existing `SAMPLE NUM_READS` output.

For tabix-style CSI, pseudo-bin or record statistics must not be reported as
read counts. VCF/BED/generic tabix records are not reads. If the output header
remains `NUM_READS`, the command should only accept formats where the count is
actually a read count.

#### Tabix aux metadata

Tabix-style CSI needs special attention because current TBI safety depends on
the index's reference names being compared with the supplied FAI. CSI's core
format treats aux as bytes, but htslib-style tabix CSI can carry the same kind
of coordinate and name metadata as TBI.

Design choices:

- Parse tabix-style aux and require name validation parity with TBI.
- Accept FAI without aux validation and document weaker safety.
- Reject tabix-style CSI when names cannot be validated.

The first option best preserves current TBI expectations. The third option is
safer than silent mislabeling if aux parsing is not viable.

#### Bare `.csi` ambiguity

The `.csi` suffix alone does not identify whether the index belongs to BAM,
CRAM, VCF, BED, or another tabix-style source. CSI magic confirms the index
format, not the semantic kind of indexed records.

The working UX assumption should be:

- Prefer recognized compound suffixes such as `.bam.csi`, `.cram.csi`,
  `.vcf.gz.csi`, and `.bed.gz.csi`.
- For bare `.csi`, use a sidecar only when it is available and can be validated
  by file magic or header.
- Do not silently guess the semantic kind when neither suffix nor sidecar
  establishes it.

There is a real downside: requiring sidecars can force access to large BAM,
CRAM, VCF, or BED payloads, which works against binest's index-only value. A
design that errors on ambiguous `.csi` may be less convenient, but a design
that guesses can be scientifically wrong without a visible failure.

#### CSI validity boundaries

Any eventual CSI path needs clear behavior for malformed geometry and future
versions. `min_shift`, `depth`, bin numbers, pseudo-bin placement, chunk
ordering, virtual-offset monotonicity, aux length, and trailing fields all
affect whether the parser can safely build a density model. These details are
not the focus of this exploration, but they shape the viability of any path.

### CRAI and CRAM

CRAI is the hardest case because it is both attractive and dangerous:

- Attractive because `.crai` files are small and index-only.
- Dangerous because the index rows describe CRAM slices, and CRAM slice bytes
  are not equivalent to read density.

#### Slice-size density

The pure index-only path would use CRAI slice size as the raw signal and map
that signal over the slice's reference span.

Benefits:

- Preserves the lightweight `.crai` workflow.
- Requires no CRAM payload download in the best case.
- Gives users some kind of CRAM-index density output.

Risks:

- Slice bytes depend on CRAM compression behavior, not just read count.
- A slice's alignment span can be much wider than its actual read footprint.
- Header and block overhead distort sparse slices.
- Different CRAM encoders or codec choices may produce different density
  signals for the same biological sample.
- Derived `chromcopy` and `sex` could diverge from BAM/BAI for reasons unrelated
  to biology.

This path is only viable for derived calls if calibration shows that the
resulting signal agrees with trusted BAM/BAI or read-count-based estimates
across representative data. Without that evidence, slice-size density should be
treated as exploratory.

#### CRAM-assisted signals

A stronger signal may require reading parts of the CRAM file.

Possible evidence levels:

- Header-only access for reference names and lengths.
- Container/slice header access for record counts and spans.
- RI data series access for multi-reference attribution.
- Record-position decoding for a true per-tile count-like signal.

This is a continuum, not a binary choice. Header-only access may be acceptable
because it is small and directly improves reference validation. RI or record
access is more scientifically useful but may require reading data blocks and
handling CRAM version/codec complexity.

The design should be explicit about where it sits on this continuum. A feature
that requires CRAM body access may still be worthwhile, but it is a different
product promise than "estimate from the small index file."

#### Multi-reference slices

Multi-reference CRAM slices are a first-class modeling problem. CRAI may list
multiple rows for the same slice, but the bytes for that slice are not cleanly
owned by one row. The CRAM spec points to RI for the actual per-record reference
ids in multi-reference slices.

Possible policies:

- RI-based attribution: read enough CRAM data to count or apportion records by
  reference. This is the most defensible but least index-only option.
- Span-based attribution: apportion by the alignment spans described in CRAI
  rows. This is index-only but is still a proxy and can be badly biased.
- Omit multi-reference slices from derived calls and surface a warning or
  unsupported status when they are nontrivial. This protects `chromcopy` and
  `sex` but loses data.
- Reject CRAI-derived calls when multi-reference slices are present.
  This is conservative and simple to explain, but less useful.

Even splitting slice bytes across rows is not scientifically justified by the
spec and should not be treated as a default answer.

#### CRAM version coverage

Supporting "CRAM" is not a single parser decision. The CRAM v3 spec names
historical versions 1.0, 2.0, 2.1, 3.0, and 3.1, with differences in EOF
markers, compression methods, checksums, and data/container details. CRAM 3.0
and 3.1 are close in structure, but 1.0 and 2.0 are older and rarer.

Design choices:

- Treat all valid CRAM versions as in scope only if fixtures and parser
  confidence exist.
- Start by scoping the design to versions with available validation material,
  while failing older versions clearly.
- Support CRAI parsing independently from CRAM body parsing, but avoid claiming
  full CRAM version support unless header/body access has the same coverage.

The document should not assume that CRAI support automatically means all CRAM
versions are equally supported for reference headers, RI, or derived signals.

### Reference and UX tradeoffs

Reference handling is where correctness and convenience pull in opposite
directions.

Robust validation wants source headers:

- BAM headers give BAI reference names and lengths.
- CRAM headers can give CRAM reference names and lengths.
- Tabix aux metadata can give names and coordinate semantics for tabix-style
  CSI.

Fast index-only QC wants small files:

- A BAI or CRAI may be cheap to inspect when the matching BAM or CRAM is not
  local.
- A remote or archived source BAM/CRAM may be too large to fetch just for a
  quick QC estimate.
- Requiring sidecars for every ambiguous case can make the new formats less
  useful than the current tool.

Possible fallback ladder:

1. Use a trusted source header when the sidecar is present and cheap enough for
   the user to provide.
2. Use index-carried names or aux metadata where the format provides them.
3. Use a supplied FAI when it can be validated against an index/header name
   source.
4. Use FAI as an unvalidated fallback only when the command can be honest about
   the risk and the user has explicitly allowed mismatch behavior.
5. Fail with a clear error when neither the index nor supplied files can safely
   label references or support build detection.

The existing `--fai`, `--reference-build`, and stdin batch behavior should stay
familiar. A clearer alias such as `--allow-reference-mismatch` may be useful,
but any such alias must preserve the existing `--allow-bam-fai-mismatch`
behavior for compatibility.

A new mandatory `--format` flag should be avoided unless suffix, sidecar, and
FAI evidence cannot resolve ambiguity safely. If ambiguity remains, an explicit
error is preferable to a silent guess.

## Compatibility constraints for any future design

These are not implementation steps; they are boundaries for evaluating any
future proposal.

- BAI and TBI behavior remain the compatibility anchor.
- No existing command should be renamed.
- No existing required flag should become mandatory for BAI/TBI users.
- Existing TSV headers should not change.
- Existing sample ordering and batch error behavior should not change.
- BAI/TBI row shape and values should not change as a side effect of adding new
  formats.
- `numreads` should remain a read-count command, not a generic record-count
  command.
- New format support should prefer suffix and magic validation over hidden
  guessing.
- When scientific confidence is lower for a format, the UX should fail or warn
  clearly rather than emit apparently equivalent biological calls.

## Open design questions

These questions should be answered before CRAM/CSI support moves from design
exploration into implementation:

1. Is fixed 16 kb projection the right normalization model for all new formats,
   or should native spans be retained with width-aware statistics?
2. For CSI, can `loffset`, chunks, or a combination of the two produce a
   density signal that is comparable to the current BAI/TBI linear-index signal?
3. How should CSI records assigned to higher-level bins be represented without
   overlap, loss, or false precision?
4. For tabix-style CSI, is aux parsing required for reference-name validation
   parity with TBI?
5. Which CSI semantic kinds should be accepted by `numreads`, given that the
   output header means reads rather than generic records?
6. Is CRAI slice-size density good enough for any derived command, or should it
   be limited to exploratory/raw output unless calibrated?
7. If CRAM body access is required, what is the minimal acceptable access level:
   header only, container/slice headers, RI, or record-position decoding?
8. How should multi-reference CRAM slices be handled if RI is unavailable or
   too expensive to read?
9. Which CRAM versions are in scope for source-header or RI-assisted behavior,
   and which should fail clearly until fixtures and parser confidence exist?
10. How should bare `.csi` be handled when there is no compound suffix and no
    sidecar available?
11. When FAI is the only reference source, what commands can safely proceed,
    and what mismatch policy should be required?
12. What user-visible wording makes CRAM/CSI support honest about proxy signals
    without making the common BAI/TBI workflow more complicated?

The most important unresolved issue is scientific viability, not file parsing.
If a proposed path can parse the indexes but cannot produce a comparable
normalization signal, it should not be treated as feature-complete support.
