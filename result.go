package binest

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"git.nygenome.org/rmusunuri/binest/internal"
)

var errInvalidAutosomeMedian = errors.New("invalid autosomal normalization median")

// ChromCopy holds the per chromosome copy estimate result for a single index
type ChromCopy struct {
	Sample   string
	Chroms   []string
	CopyNums []uint8
	NormEsts []float64
}

func (c *ChromCopy) String() string {
	lines := make([]string, len(c.Chroms))

	// header -> "index_used\tchrom\tcopy_estimate\tnormalized_estimate"
	for idx, chrom := range c.Chroms {
		lines[idx] = fmt.Sprintf("%s\t%s\t%d\t%s", c.Sample, chrom, c.CopyNums[idx],
			strconv.FormatFloat(c.NormEsts[idx], 'f', -1, 64))
	}

	return strings.Join(lines, "\n")
}

// Sex holds the sex genotype estimate result for a single index
type Sex struct {
	Sample   string
	Gender   string
	Genotype string
	NormXEst float64
	NormYEst float64
}

func (s *Sex) String() string {
	// header -> "#index_used\testimated_gender\tsex_genotype\tnormalized_x_estimate\tnormalized_y_estimate"
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s", s.Sample, s.Gender, s.Genotype,
		strconv.FormatFloat(s.NormXEst, 'f', -1, 64),
		strconv.FormatFloat(s.NormYEst, 'f', -1, 64))
}

// Sizes holds the per window size size estimates result for a single index
type Sizes struct {
	Sample   string
	Chroms   []string
	Starts   [][]uint64
	RawSizes [][]int64
	NormEsts [][]float64
}

// Normalize normalizes the raw sizes read from the index.
func (s *Sizes) Normalize() error {
	medianBinSize, err := autosomalMedian(s.Chroms, s.RawSizes)
	if err != nil {
		return err
	}

	s.NormEsts = make([][]float64, len(s.Chroms))

	// Normalize by dividing per bin byte size by autosomes byte size median
	for refID, refSizes := range s.RawSizes {
		s.NormEsts[refID] = make([]float64, len(refSizes))
		for binIdx, binRawSize := range refSizes {
			s.NormEsts[refID][binIdx] = float64(binRawSize) / medianBinSize
		}
	}
	return nil
}

func autosomalMedian(chroms []string, rawSizes [][]int64) (float64, error) {
	if len(chroms) != len(rawSizes) {
		return 0, fmt.Errorf("%w: chromosome and raw-size counts differ", errInvalidAutosomeMedian)
	}

	vals := make([]int64, 0, 200000)
	for idx, refSizes := range rawSizes {
		for binIdx, rawSize := range refSizes {
			if rawSize < 0 {
				return 0, fmt.Errorf("%w: negative raw size for chrom %s bin %d", errMalformedIndex, chroms[idx], binIdx)
			}
		}
		if isSexChrom(chroms[idx]) {
			continue
		}
		vals = append(vals, refSizes...)
	}
	if len(vals) == 0 {
		return 0, fmt.Errorf("%w: no usable autosomal bins", errInvalidAutosomeMedian)
	}

	median, err := medianI64(vals)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", errInvalidAutosomeMedian, err)
	}
	if !isPositiveFinite(median) {
		return 0, fmt.Errorf("%w: got %s", errInvalidAutosomeMedian, strconv.FormatFloat(median, 'f', -1, 64))
	}
	return median, nil
}

func chromMedianOrZero(rawSizes []int64) (float64, error) {
	if len(rawSizes) == 0 {
		return 0, nil
	}

	median, err := medianI64(append([]int64(nil), rawSizes...))
	if err != nil {
		return 0, err
	}
	if median == 0 {
		return 0, nil
	}
	if median < 0 || !isFinite(median) {
		return 0, fmt.Errorf("invalid chromosome median: got %s", strconv.FormatFloat(median, 'f', -1, 64))
	}
	return median, nil
}

func isSexChrom(chrom string) bool {
	return chrom == "X" || chrom == "Y" || chrom == "chrX" || chrom == "chrY"
}

func isPositiveFinite(v float64) bool {
	return v > 0 && isFinite(v)
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func (s *Sizes) String() string {
	lines := make([]string, 0, 200000)

	if len(s.NormEsts) > 0 {
		// norm ests print
		// header -> "#chrom\tstart\tend\t%normalized_size\tindex_used"
		for refId, chrom := range s.Chroms {
			for binIdx, start := range s.Starts[refId] {
				lines = append(lines, fmt.Sprintf("%s\t%d\t%d\t%s\t%s",
					chrom, start, start+internal.TileWidth,
					strconv.FormatFloat(s.NormEsts[refId][binIdx], 'f', -1, 64), s.Sample))
			}
		}
		return strings.Join(lines, "\n")
	}

	// raw sizes print
	// header -> "#chrom\tstart\tend\t%raw_size\tindex_used"
	for refId, chrom := range s.Chroms {
		for binIdx, start := range s.Starts[refId] {
			lines = append(lines, fmt.Sprintf("%s\t%d\t%d\t%d\t%s",
				chrom, start, start+internal.TileWidth, s.RawSizes[refId][binIdx], s.Sample))
		}
	}

	return strings.Join(lines, "\n")
}
