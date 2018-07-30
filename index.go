package binest

import (
	"path/filepath"
	"regexp"
	"strings"

	"git.nygenome.org/rmusunuri/binest/internal"
)

var (
	excludeChroms = regexp.MustCompile("^MT$|^chrM$|^GL|^chrUn|^chrEBV|^HLA-|_random$|_alt$|_decoy$")
)

// Index holds the raw bins and the refmap for a bai/tbi index
type Index struct {
	Bins   *Bins
	RefMap *RefMap
	SrcIdx string
}

// ChromCopy estimates the per chomosome copy number for the given index
func (i *Index) ChromCopy(ploidy uint) *ChromCopy {
	normBins := i.Sizes(false)
	normCopies := make([]float64, len(normBins.Chroms))
	estCopies := make([]uint8, len(normBins.Chroms))

	for refID := range normBins.Chroms {
		normCopies[refID] = float64(ploidy) * medianF64(normBins.NormEsts[refID])
		estCopies[refID] = roundChromSize(normCopies[refID])
	}

	copies := ChromCopy{
		IdxUsed:  i.SrcIdx,
		Chroms:   normBins.Chroms,
		CopyNums: estCopies,
		NormEsts: normCopies,
	}

	return &copies
}

// Sex estimates the sex genotype for the given index
func (i *Index) Sex(ploidy uint) *Sex {
	normBins := i.Sizes(false)

	var (
		xCopy  uint8
		yCopy  uint8
		xNorm  float64
		yNorm  float64
		gender string
		sexGT  string
	)

	for refID, chrom := range normBins.Chroms {
		if strings.HasSuffix(chrom, "X") {
			xNorm = float64(ploidy) * medianF64(normBins.NormEsts[refID])
			xCopy = roundChromSize(xNorm)
		}
		if strings.HasSuffix(chrom, "Y") {
			yNorm = float64(ploidy) * medianF64(normBins.NormEsts[refID])
			yCopy = roundChromSize(yNorm)
		}
	}

	if xCopy > 3 {
		xCopy = 3
	}
	if yCopy > 3 {
		yCopy = 3
	}

	sexGT = strings.Repeat("X", int(xCopy)) + strings.Repeat("Y", int(yCopy))
	if len(sexGT) == 1 && int(yCopy) == 0 {
		sexGT = "XO"
	}
	if len(sexGT) == 1 && int(xCopy) == 0 {
		sexGT = "OY"
	}

	switch sexGT {
	case "XX":
		gender = "female"
	case "XY":
		gender = "male"
	default:
		gender = "unknown"
	}

	return &Sex{
		IdxUsed:  i.SrcIdx,
		Gender:   gender,
		Genotype: sexGT,
		NormXEst: xNorm,
		NormYEst: yNorm,
	}
}

// Sizes estimates the raw/normalized per bin sizes for the given index
func (i *Index) Sizes(rawSize bool) *Sizes {
	chroms := make([]string, 0, len(*i.Bins))
	starts := make([][]uint64, 0, len(*i.Bins))
	rawBins := make([][]int64, 0, len(*i.Bins))

	var (
		position   uint64
		foundChrom bool
		chromName  string
	)

	for refID, refBins := range *i.Bins {
		position = 0

		chromName, foundChrom = (*i.RefMap)[refID]
		if !foundChrom || excludeChroms.MatchString(chromName) {
			// skip chromosome if not found in refmap or if found in exclude regex
			continue
		}

		chroms = append(chroms, chromName)
		rawBins = append(rawBins, refBins)
		starts = append(starts, make([]uint64, len(refBins)))
		startIdx := len(starts) - 1
		for binIdx := range refBins {
			starts[startIdx][binIdx] = position
			position += internal.TileWidth
		}
	}

	data := &Sizes{
		IdxUsed:  i.SrcIdx,
		Chroms:   chroms,
		Starts:   starts,
		RawSizes: rawBins,
		NormEsts: [][]float64{},
	}

	if rawSize {
		return data
	}

	data.Normalize()
	return data
}

// NewIndex builds a new index given the path to index file and optionally path to reference fasta index.
func NewIndex(idxPath, faiPath string) (*Index, error) {
	srcbase := filepath.Base(idxPath)
	refmap, err := ReadRefMap(idxPath, faiPath)
	if err != nil {
		return nil, err
	}

	bins, err := ReadBins(idxPath, refmap.GenomeBuild())
	if err != nil {
		return nil, err
	}

	return &Index{Bins: bins, RefMap: refmap, SrcIdx: srcbase}, nil
}
