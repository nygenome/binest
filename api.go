package binest

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"unsafe"

	"github.com/biogo/hts/bam"
	"github.com/biogo/hts/bgzf"
	"github.com/biogo/hts/sam"
	"github.com/biogo/store/interval"
)

// Common errors which will be raised in the package.
var (
	ErrTooManySamples        = errors.New("binest: Too many samples in RG header")
	ErrNoChunksToUse         = errors.New("binest: No usable chunks for estimates")
	ErrNotEnoughBins         = errors.New("binest: Not enough bins availabe to use")
	ErrNegativeVirtualOffset = errors.New("binest: Non positive change in vOffset")
)

// refIndex is the index of a single reference.
type refIndex struct {
	bins      []bin
	stats     *referenceStats
	intervals []bgzf.Offset
}

// bin represents a BAM index bin.
type bin struct {
	bin    uint32
	chunks []bgzf.Chunk
}

// referenceStats holds mapping statistics available in the BAM index.
type referenceStats struct {
	chunk    bgzf.Chunk
	mapped   uint64
	unmapped uint64
}

// RefBlock is a chunk in the genome implementing biogo's IntInterface
type RefBlock struct {
	Name  string
	RefID int
	Start int
	End   int
}

// ID returns the unique ID for the RefBlock
func (r *RefBlock) ID() uintptr {
	return uintptr(r.RefID)
}

// Range gives the range of the current RefBlock
func (r *RefBlock) Range() interval.IntRange {
	return interval.IntRange{Start: r.Start, End: r.End}
}

// Overlap returns a boolean indicating whether the receiver overlaps a range
func (r *RefBlock) Overlap(b interval.IntRange) bool {
	return r.End > b.Start && r.Start < b.End
}

// rawBin holds the raw size and location of a single bin
type rawBin struct {
	Size  int64
	Chunk bgzf.Chunk
}

// NormBin has the normalized size and location of a single bin
type NormBin struct {
	Size  float64
	Chunk bgzf.Chunk
}

// RawBinData represents the raw bin data of a sample
type RawBinData struct {
	Bins   map[RefBlock]rawBin
	Blocks []RefBlock
}

// NormBinData represents the normalized bin data of a sample
type NormBinData struct {
	Bins   map[RefBlock]NormBin
	Blocks []RefBlock
}

// SampleIndex holds relevant information to operate in BAM index bins.
type SampleIndex struct {
	Name   string
	Index  *bam.Index
	RefMap map[int]*sam.Reference
}

// bins returns the bin size and BGZF chunks for each chunk from the bam index
func (s *SampleIndex) bins() ([][]rawBin, error) {
	idxRefs := reflect.ValueOf(*s.Index).FieldByName("idx").FieldByName("Refs")
	idxRefsPtr := unsafe.Pointer(idxRefs.Pointer())
	refIdxs := (*(*[1 << 32]refIndex)(idxRefsPtr))[:idxRefs.Len()]

	var (
		binSize       int64
		binChunk      bgzf.Chunk
		intervalBegin bgzf.Offset
	)

	bins := make([][]rawBin, len(refIdxs))

	for refNum, rIdx := range refIdxs {
		// Ignore chromosomes too small to hold a chunk
		if len(rIdx.intervals) < 2 {
			bins[refNum] = make([]rawBin, 0)
			continue
		}

		bins[refNum] = make([]rawBin, len(rIdx.intervals)-1)
		for chunkNum, intervalEnd := range rIdx.intervals[1:] {
			intervalBegin = rIdx.intervals[chunkNum]
			binSize = VOffset(intervalEnd) - VOffset(intervalBegin)
			binChunk = bgzf.Chunk{Begin: intervalBegin, End: intervalEnd}

			bins[refNum][chunkNum] = rawBin{Size: binSize, Chunk: binChunk}

			if binSize < 0 {
				panic(ErrNegativeVirtualOffset)
			}
		}

		rIdx.bins, rIdx.intervals = nil, nil
	}

	return bins, nil
}

// RawBins returns the raw bin data for the sample
func (s *SampleIndex) RawBins() (RawBinData, error) {
	bins, err := s.bins()
	if err != nil {
		return RawBinData{}, err
	}

	rawBins := make(map[RefBlock]rawBin)
	refBlocks := make([]RefBlock, 0, 65536)

	var (
		pos    int
		rName  string
		rBlock RefBlock
	)

	for _, ref := range s.RefMap {
		pos = 0
		rName = ref.Name()
		binsForRef := bins[ref.ID()]
		for _, b := range binsForRef {
			rBlock = RefBlock{RefID: ref.ID(), Start: pos, End: pos + 16384, Name: rName}
			rawBins[rBlock] = rawBin{Size: b.Size, Chunk: b.Chunk}
			pos += 16384
			refBlocks = append(refBlocks, rBlock)
		}
	}

	return RawBinData{Bins: rawBins, Blocks: refBlocks}, nil
}

// NormalizedBins returns the normalized BinData for the sample
func (s *SampleIndex) NormalizedBins() (NormBinData, error) {
	bins, err := s.bins()
	if err != nil {
		return NormBinData{}, err
	}

	mergedBinSizes := make([]int64, 0, 65536)

	for i := 0; i < len(bins); i++ {
		for j := 0; j < len(bins[i]); j++ {
			mergedBinSizes = append(mergedBinSizes, bins[i][j].Size)
		}
	}

	fmt.Fprintf(os.Stderr, "Found %d usable bins in the index\n", len(mergedBinSizes))
	if len(mergedBinSizes) < 1024 {
		return NormBinData{}, ErrNotEnoughBins
	}

	medianBinSize := MedianInt64(mergedBinSizes)

	var (
		pos    int
		rName  string
		normed float64
		rBlock RefBlock
	)

	normedBins := make(map[RefBlock]NormBin)
	refBlocks := make([]RefBlock, 0, 65536)

	for _, ref := range s.RefMap {
		pos = 0
		rName = ref.Name()
		binsForRef := bins[ref.ID()]
		for _, b := range binsForRef {
			rBlock = RefBlock{RefID: ref.ID(), Start: pos, End: pos + 16384, Name: rName}
			normed = float64(b.Size) / medianBinSize
			normedBins[rBlock] = NormBin{Size: normed, Chunk: b.Chunk}
			pos += 16384
			refBlocks = append(refBlocks, rBlock)
		}
	}

	return NormBinData{Bins: normedBins, Blocks: refBlocks}, nil
}

// Stats returns the number of mapped reads and total number of bases in the genome
func (s *SampleIndex) Stats() (uint64, uint64) {
	var mappedReads, genomeBases uint64

	for refID, ref := range s.RefMap {
		genomeBases += uint64(ref.Len())
		refStats, ok := s.Index.ReferenceStats(refID)
		if !ok {
			msg := "chrom %s found in BAM header but missing in BAM index for sample %s\n"
			fmt.Fprintf(os.Stderr, msg, ref.Name(), s.Name)
			continue
		}
		mappedReads += refStats.Mapped
	}

	return mappedReads, genomeBases
}

// NewSampleIndex returns a new SampleIndex using the given bam index and header
func NewSampleIndex(idx *bam.Index, hdr *sam.Header) (*SampleIndex, error) {
	var sample string
	samples := make(map[string]bool)
	refMap := make(map[int]*sam.Reference)

	for _, rg := range hdr.RGs() {
		sample = rg.Get(sam.NewTag("SM"))
		samples[sample] = true
	}

	if len(samples) != 1 {
		return nil, ErrTooManySamples
	}

	for _, ref := range hdr.Refs() {
		refMap[ref.ID()] = ref
	}

	return &SampleIndex{Name: sample, Index: idx, RefMap: refMap}, nil
}
