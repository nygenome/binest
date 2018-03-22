package binest

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/omicsnut/binest/internal"
	"github.com/pkg/errors"
)

// BinData holds the binSizes, RefMap and Name of one sample
type BinData struct {
	Name     string
	IdxType  IndexType
	binSizes [][]int64
	cache    map[string][]Bin
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

// SexEstimate holds the gender, genotype, X and Y copies for a sample
type SexEstimate struct {
	Name   string
	Gender string
	SexGT  string
	XCopy  uint32
	YCopy  uint32
	XNorm  float64
	YNorm  float64
}

// Raw returns the raw bins for the sample
func (bd *BinData) Raw(refMap map[uint32]string) []Bin {
	if bins, ok := bd.cache["raw"]; ok {
		return bins
	}

	bins := make([]Bin, 0, 200000)

	var (
		found     bool
		position  uint32
		chromName string
	)

	excludeChroms := regexp.MustCompile("^GL|^chrUn|^chrEBV|^HLA-|_random$|_alt$|_decoy$")

	for refID, refBins := range bd.binSizes {
		position = 0

		if chromName, found = refMap[uint32(refID)]; !found {
			continue
		}

		if excludeChroms.MatchString(chromName) {
			continue
		}

		for _, binSize := range refBins {
			bins = append(bins, Bin{
				Ref:   chromName,
				Start: position,
				End:   position + internal.TileWidth,
				Size:  float64(binSize),
			})
			position += internal.TileWidth
		}
	}

	bd.cache["raw"] = bins
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
func (bd *BinData) Normalized(refMap map[uint32]string) []Bin {
	if bins, ok := bd.cache["norm"]; ok {
		return bins
	}

	bins := make([]Bin, 0, 200000)
	medianSize := bd.medianBinSize()

	var (
		found     bool
		normSize  float64
		position  uint32
		chromName string
	)

	for refID, refBins := range bd.binSizes {
		position = 0
		for _, binSize := range refBins {
			normSize = float64(binSize) / medianSize

			if math.IsNaN(normSize) {
				position += internal.TileWidth
				continue
			}

			if chromName, found = refMap[uint32(refID)]; !found {
				position += internal.TileWidth
				continue
			}

			bins = append(bins, Bin{
				Ref:   chromName,
				Start: position,
				End:   position + internal.TileWidth,
				Size:  normSize,
			})
			position += internal.TileWidth
		}
	}

	bd.cache["norm"] = bins
	return bins
}

// Copies returns the copy number estimates for all references in the sample
func (bd *BinData) Copies(ploidy uint, refMap map[uint32]string) []RefCopy {
	normBins := bd.Normalized(refMap)

	prevRef := normBins[0].Ref
	refSizes := make([]float64, 0, 20000)
	copies := make([]RefCopy, 0, 100)

	var normCopy float64
	for _, b := range normBins {
		if b.Ref != prevRef {
			normCopy = float64(ploidy) * medianF64(refSizes)
			copies = append(copies, RefCopy{prevRef, roundChromSize(normCopy), normCopy})
			refSizes = make([]float64, 0, 200000)
		}
		refSizes = append(refSizes, b.Size)
		prevRef = b.Ref
	}

	return copies
}

// DetectSex returns the SexEstimate for the sample from the bindata
func (bd *BinData) DetectSex(ploidy uint, refMap map[uint32]string) SexEstimate {
	copies := bd.Copies(ploidy, refMap)

	var (
		xCopy  uint32
		yCopy  uint32
		xNorm  float64
		yNorm  float64
		gender string
		sexGT  string
	)

	for _, c := range copies {
		if strings.HasSuffix(c.Ref, "X") {
			xCopy, xNorm = c.Est, c.Norm
			if xCopy > uint32(4) {
				xCopy = uint32(4)
			}
		}
		if strings.HasSuffix(c.Ref, "Y") {
			yCopy, yNorm = c.Est, c.Norm
			if yCopy > uint32(4) {
				yCopy = uint32(4)
			}
		}
	}

	sexGT = strings.Repeat("X", int(xCopy)) + strings.Repeat("Y", int(yCopy))

	switch {
	case xCopy == uint32(2) && yCopy == uint32(0):
		gender = "female"
	case xCopy == uint32(1) && yCopy == uint32(1):
		gender = "male"
	default:
		gender = "unknown"
	}

	return SexEstimate{bd.Name, gender, sexGT, xCopy, yCopy, xNorm, yNorm}
}

// NewBinData returns BinData given path to a BAI/TBI and reference FAI index
func NewBinData(idxPath string) (*BinData, error) {
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
		return nil, err
	}

	cache := make(map[string][]Bin, 2)

	return &BinData{name, idxType, binSizes(rIdxs), cache}, nil
}

// String implements the fmt.Stringer interface for SexEstimate
func (s SexEstimate) String() string {
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s", s.Name, s.Gender, s.SexGT,
		strconv.FormatFloat(s.XNorm, 'f', -1, 64),
		strconv.FormatFloat(s.YNorm, 'f', -1, 64))
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
