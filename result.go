package binest

import (
	"fmt"
	"strconv"
	"strings"

	"git.nygenome.org/rmusunuri/binest/internal"
)

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

// Normalize normalizes the raw sizes read from the index
func (s *Sizes) Normalize() {
	s.NormEsts = make([][]float64, len(s.Chroms))

	vals := make([]int64, 0, 200000)
	for _, refSizes := range s.RawSizes {
		vals = append(vals, refSizes...)
	}
	medianBinSize := medianI64(vals)

	for refID, refSizes := range s.RawSizes {
		s.NormEsts[refID] = make([]float64, len(refSizes))
		for binIdx, binRawSize := range refSizes {
			s.NormEsts[refID][binIdx] = float64(binRawSize) / medianBinSize
		}
	}
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
