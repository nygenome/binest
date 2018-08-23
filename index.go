package binest

import (
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
	Sample string
}

// ChromCopy estimates the per chomosome copy number for the given index
func (i *Index) ChromCopy(ploidy uint) *ChromCopy {
	rawBins := i.Sizes(true)

	// compute median byte size in autosomes
	vals := make([]int64, 0, 200000)
	for idx, refSizes := range rawBins.RawSizes {
		if rawBins.Chroms[idx] == "X" || rawBins.Chroms[idx] == "Y" || rawBins.Chroms[idx] == "chrX" || rawBins.Chroms[idx] == "chrY" {
			continue
		}
		vals = append(vals, refSizes...)
	}
	autoMedianSize := medianI64(vals)

	normCopies := make([]float64, len(rawBins.Chroms))
	estCopies := make([]uint8, len(rawBins.Chroms))

	// divide per chromosome median byte size by the autosome
	// median byte size to get approx. copy number for chrom.
	for idx, refSizes := range rawBins.RawSizes {
		refMedianSize := medianI64(refSizes)
		normCopies[idx] = float64(ploidy) * refMedianSize / autoMedianSize
		estCopies[idx] = roundChromSize(normCopies[idx])
	}

	copies := ChromCopy{
		Sample:   i.Sample,
		Chroms:   rawBins.Chroms,
		CopyNums: estCopies,
		NormEsts: normCopies,
	}

	return &copies
}

// Sex estimates the sex genotype for the given index
func (i *Index) Sex(ploidy uint) *Sex {
	rawBins := i.Sizes(true)

	// compute median byte size in autosomes
	vals := make([]int64, 0, 200000)
	for idx, refSizes := range rawBins.RawSizes {
		if rawBins.Chroms[idx] == "X" || rawBins.Chroms[idx] == "Y" || rawBins.Chroms[idx] == "chrX" || rawBins.Chroms[idx] == "chrY" {
			continue
		}
		vals = append(vals, refSizes...)
	}
	autoMedianSize := medianI64(vals)

	var (
		xCopy  uint8
		yCopy  uint8
		xNorm  float64
		yNorm  float64
		gender string
		sexGT  string
	)

	// divide per chromosome median byte size by the autosome
	// median byte size to get approx. copy number for chrom.
	for idx, refSizes := range rawBins.RawSizes {
		if rawBins.Chroms[idx] == "X" || rawBins.Chroms[idx] == "chrX" {
			refMedianSize := medianI64(refSizes)
			xNorm = float64(ploidy) * (refMedianSize / autoMedianSize)
			xCopy = roundChromSize(xNorm)
		}
		if rawBins.Chroms[idx] == "Y" || rawBins.Chroms[idx] == "chrY" {
			refMedianSize := medianI64(refSizes)
			yNorm = float64(ploidy) * (refMedianSize / autoMedianSize)
			yCopy = roundChromSize(yNorm)
		}
	}

	if xCopy > 3 {
		xCopy = 3
	}
	if yCopy > 3 {
		yCopy = 3
	}
	// When XO and yNorm is between 0.25 and 0.7, call "XO/XY"
	if xCopy == 1 && yCopy == 0 && yNorm >= 0.25 && yNorm < 0.7 {
		yCopy = 1
		sexGT = "XO/XY"
	}
	sexGT = strings.Repeat("X", int(xCopy)) + strings.Repeat("Y", int(yCopy))

	switch sexGT {
	case "XX":
		gender = "female"
	case "XY", "XO/XY":
		gender = "male"
	default:
		gender = "unknown"
	}

	return &Sex{
		Sample:   i.Sample,
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
		currBins := make([]int64, 0, len(refBins))
		currStarts := make([]uint64, 0, len(refBins))
		for _, binSize := range refBins {
			if binSize > 0 {
				currBins = append(currBins, binSize)
				currStarts = append(currStarts, position)
			}
			position += internal.TileWidth
		}

		rawBins = append(rawBins, currBins)
		starts = append(starts, currStarts)
	}

	data := &Sizes{
		Sample:   i.Sample,
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
	sample := stripKnownSuffixes(idxPath)
	refmap, err := ReadRefMap(idxPath, faiPath)
	if err != nil {
		return nil, err
	}

	bins, err := ReadBins(idxPath, refmap.GenomeBuild())
	if err != nil {
		return nil, err
	}

	return &Index{Bins: bins, RefMap: refmap, Sample: sample}, nil
}
