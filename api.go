package binest

import (
	"errors"
	"reflect"
	"unsafe"

	"github.com/biogo/hts/bam"
	"github.com/biogo/hts/bgzf"
	"github.com/biogo/hts/sam"
)

const (
	// TileWidth is the length of the interval tiling used in BAI and tabix indexes.
	TileWidth = 0x4000

	// StatsDummyBin is the bin number of the reference statistics bin used in BAI and tabix indexes.
	StatsDummyBin = 0x924a
)

var (
	// ErrTooManySamples when more than one unique sample found in RG header
	ErrTooManySamples = errors.New("binest: Too many samples in RG header")
	// ErrNegativeVirtualOffset when non positive change in virtual offset between start and end of bins
	ErrNegativeVirtualOffset = errors.New("binest: Non positive change in vOffset")
	// ErrNoChunksInBam when not enough usable bins in the BAM index
	ErrNoChunksInBam = errors.New("binest: No usable chunks found in BAM index")
)

// RefIndex is the index of a single reference.
type RefIndex struct {
	Bins      []Bin
	Stats     *ReferenceStats
	Intervals []bgzf.Offset
}

// Bin represents a BAM index bin.
type Bin struct {
	Bin    uint32
	Chunks []bgzf.Chunk
}

// ReferenceStats holds mapping statistics for a genomic reference.
type ReferenceStats struct {
	// Chunk is the span of the indexed BGZF
	// holding alignments to the reference.
	Chunk bgzf.Chunk

	// Mapped is the count of mapped reads.
	Mapped uint64

	// Unmapped is the count of unmapped reads.
	Unmapped uint64
}

// BinType represents the size class of the bin unit
type BinType int

const (
	// VeryLow has binSize -> <= 0.2 * median
	VeryLow BinType = iota
	// Low has binSize -> 0.2 * median < binSize <= 0.8 * median
	Low
	// Normal has binSize -> 0.8 * median < binSize <= 1.2 * median
	Normal
	// High has binSize -> 1.2 * median < binSize <= 1.7 * median
	High
	// VeryHigh has binSize -> binSize > 1.7 * median
	VeryHigh
)

// RefBlock represents a chunk in the genome
type RefBlock struct {
	RefID int
	Start int
	End   int
}

// BinUnit has the size and location of a single bin in the index
type BinUnit struct {
	Size  int64
	Type  BinType
	Chunk bgzf.Chunk
	Block RefBlock
}

// BinData holds the sizes and location of all the bins in the index.
type BinData struct {
	Units  []BinUnit
	Median float64
}

// SampleIndex holds relevant information to operate in BAM index bins.
type SampleIndex struct {
	Name   string
	Index  *bam.Index
	RefMap map[int]*sam.Reference
}

// getBinType takes a bin size and returns the BinType
func getBinType(val, median float64) BinType {
	scaled := val / median
	if scaled <= float64(0.2) {
		return VeryLow
	} else if scaled > float64(0.2) && scaled <= float64(0.8) {
		return Low
	} else if scaled > float64(0.8) && scaled <= float64(1.2) {
		return Normal
	} else if scaled > float64(1.2) && scaled <= float64(1.7) {
		return High
	} else {
		return VeryHigh
	}
}

// BinSizes returns the BinData for SampleIndex
func (s *SampleIndex) BinSizes() (*BinData, error) {
	idxRefs := reflect.ValueOf(*s.Index).FieldByName("idx").FieldByName("Refs")
	idxRefsPtr := unsafe.Pointer(idxRefs.Pointer())
	refIdxs := (*(*[1 << 30]RefIndex)(idxRefsPtr))[:idxRefs.Len()]

	bins := make([][]int64, len(refIdxs))
	offsets := make([][]bgzf.Chunk, len(refIdxs))
	for i, refidx := range refIdxs {
		if len(refidx.Intervals) < 2 {
			bins[i] = make([]int64, 0)
			offsets[i] = make([]bgzf.Chunk, 0)
			continue
		}
		bins[i] = make([]int64, len(refidx.Intervals)-1)
		offsets[i] = make([]bgzf.Chunk, len(refidx.Intervals)-1)
		for j, endOff := range refidx.Intervals[1:] {
			beginOff := refidx.Intervals[j]
			bins[i][j] = vOffset(endOff) - vOffset(beginOff)
			offsets[i][j] = bgzf.Chunk{Begin: beginOff, End: endOff}
			if bins[i][j] < 0 {
				return nil, ErrNegativeVirtualOffset
			}
		}
		refidx.Bins, refidx.Intervals = nil, nil
	}

	mergedSizes := make([]int64, 0, 0x4000)
	for i := 0; i < len(bins); i++ {
		mergedSizes = append(mergedSizes, bins[i]...)
	}
	if len(mergedSizes) == 0 {
		return nil, ErrNoChunksInBam
	}

	medianSize := Median(mergedSizes)
	binUnits := make([]BinUnit, len(mergedSizes))

	var chromPos int
	var rBlock RefBlock

	for rid := 0; rid < len(bins); rid++ {
		chromPos = 0
		for cid := 0; cid < len(bins[rid]); cid++ {

			rBlock = RefBlock{
				RefID: rid,
				Start: chromPos,
				End:   chromPos + 16384,
			}

			chromPos += 16384

			binUnits = append(binUnits, BinUnit{
				Block: rBlock,
				Size:  bins[rid][cid],
				Chunk: offsets[rid][cid],
				Type:  getBinType(float64(bins[rid][cid]), medianSize),
			})
		}
	}

	return &BinData{Units: binUnits, Median: medianSize}, nil
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
