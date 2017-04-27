package binest

import (
	"fmt"
	"strconv"

	"github.com/omicsnut/binest/internal"
	"github.com/pkg/errors"
)

// BinData holds the binSizes, RefMap and Name of one sample
type BinData struct {
	Name     string
	IdxType  IndexType
	binSizes [][]int64
	refMap   map[uint32]string
}

// Bin holds the size and refblock for a reference bin
type Bin struct {
	Ref   string
	Start uint32
	End   uint32
	Size  float64
}

// RefCopy holds the copy number estimate of a reference
type RefCopy struct {
	Ref  string
	Est  uint32
	Norm float64
}

// Raw returns the raw bins for the sample
func (bd *BinData) Raw() []Bin {
	bins := make([]Bin, 0, 200000)

	var position uint32
	for refID, refBins := range bd.binSizes {
		position = 0
		for _, binSize := range refBins {
			bins = append(bins, Bin{
				Ref:   bd.refMap[uint32(refID)],
				Start: position,
				End:   position + internal.TileWidth,
				Size:  float64(binSize),
			})
			position += internal.TileWidth
		}
	}

	return bins
}

// medianBinSize returns the median size of all bins in the sample
func (bd *BinData) medianBinSize() float64 {
	tmpMerged := make([]int64, 0, 200000)

	for i := 0; i < len(bd.binSizes); i++ {
		tmpMerged = append(tmpMerged, bd.binSizes[i]...)
	}

	return medianI64(tmpMerged)
}

// Normalized returns the normalized bins for the sample
func (bd *BinData) Normalized() []Bin {
	bins := make([]Bin, 0, 200000)
	medianSize := bd.medianBinSize()

	var position uint32
	for refID, refBins := range bd.binSizes {
		position = 0
		for _, binSize := range refBins {
			bins = append(bins, Bin{
				Ref:   bd.refMap[uint32(refID)],
				Start: position,
				End:   position + internal.TileWidth,
				Size:  float64(binSize) / medianSize,
			})
			position += internal.TileWidth
		}
	}

	return bins
}

// Copies returns the copy number estimates for all references in the sample
func (bd *BinData) Copies(ploidy int) []RefCopy {
	normBins := bd.Normalized()

	prevRef := normBins[0].Ref
	refSizes := make([]float64, 0, 20000)
	copies := make([]RefCopy, 0, 100)

	var (
		normCopy float64
		estCopy  uint32
	)

	for _, b := range normBins {
		if b.Ref != prevRef {
			switch {
			case len(refSizes) > 2:
				normCopy = float64(ploidy) * medianF64(refSizes)
			default:
				normCopy = float64(ploidy) * meanF64(refSizes)
			}
			estCopy = uint32(roundF64(normCopy, 0.7, 0))
			copies = append(copies, RefCopy{prevRef, estCopy, normCopy})
			refSizes = make([]float64, 0, 200000)
		}
		refSizes = append(refSizes, b.Size)
		prevRef = b.Ref
	}

	return copies
}

// NewBinData returns BinData given path to a BAI/TBI and reference FAI index
func NewBinData(idxPath, faiPath string) (*BinData, error) {
	name, idxType := detectIndex(idxPath)

	var (
		err   error
		rIdxs []internal.RefIndex
	)

	switch idxType {
	case BAI:
		rIdxs, err = baiRefIdxs(idxPath)
	case TBI:
		rIdxs, err = tbiRefIdxs(idxPath)
	default:
		err = errors.Errorf("index file %s must be a .bai or .tbi", idxPath)
	}

	if err != nil {
		return nil, errors.Wrap(err, "Error unknown index type")
	}

	refs, err := getRefMap(faiPath)
	if err != nil {
		return nil, errors.Wrap(err, "Error fetching reference data")
	}

	return &BinData{name, idxType, binSizes(rIdxs), refs}, nil
}

// String implements the fmt.Stringer interface for RefCopy
func (rc RefCopy) String() string {
	return fmt.Sprintf("%s\t%d\t%s", rc.Ref, rc.Est,
		strconv.FormatFloat(rc.Norm, 'f', -1, 64))
}

// String implements the fmt.Stringer interface for Bin
func (b Bin) String() string {
	return fmt.Sprintf("%s\t%d\t%d\t%s", b.Ref, b.Start, b.End,
		strconv.FormatFloat(b.Size, 'f', -1, 64))
}
