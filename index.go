package binest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nygenome/binest/internal"
)

var (
	excludeChroms = regexp.MustCompile("^MT$|^chrM$|^GL|^chrUn|^chrEBV|^HLA-|_random$|_alt$|_decoy$")
)

// Index holds the raw bins and the refmap for a bai/tbi index
type Index struct {
	Bins           *Bins
	RefMap         *RefMap
	RefLengths     *RefLengths
	ReferenceBuild ReferenceBuild
	Sample         string
}

// ChromCopy estimates the per chomosome copy number for the given index
func (i *Index) ChromCopy(ploidy uint) (*ChromCopy, error) {
	rawBins, err := i.Sizes(true)
	if err != nil {
		return nil, err
	}

	autoMedianSize, err := autosomalMedian(rawBins.Chroms, rawBins.RawSizes)
	if err != nil {
		return nil, err
	}

	normCopies := make([]float64, len(rawBins.Chroms))
	estCopies := make([]uint8, len(rawBins.Chroms))

	// divide per chromosome median byte size by the autosome
	// median byte size to get approx. copy number for chrom.
	for idx, refSizes := range rawBins.RawSizes {
		refMedianSize, err := chromMedianOrZero(refSizes)
		if err != nil {
			return nil, fmt.Errorf("chromosome %s: %w", rawBins.Chroms[idx], err)
		}
		normCopies[idx] = float64(ploidy) * refMedianSize / autoMedianSize
		estCopies[idx] = roundChromSize(normCopies[idx])
	}

	copies := ChromCopy{
		Sample:   i.Sample,
		Chroms:   rawBins.Chroms,
		CopyNums: estCopies,
		NormEsts: normCopies,
	}

	return &copies, nil
}

func estimateSex(xNorm, yNorm float64, forceMF bool) *Sex {
	xCopy := roundChromSize(xNorm)
	yCopy := roundChromSize(yNorm)

	if xCopy > 3 {
		xCopy = 3
	}
	if yCopy > 3 {
		yCopy = 3
	}

	sexGT := strings.Repeat("X", int(xCopy)) + strings.Repeat("Y", int(yCopy))

	// When XO and yNorm is < 0.25 and XNorm > 1.5, call "XX"
	if xCopy == 1 && yCopy == 0 && yNorm < 0.25 && xNorm > 1.5 {
		sexGT = "XX"
	}

	// When XO and yNorm is between 0.25 and 0.7, call "XO/XY"
	if xCopy == 1 && yCopy == 0 && yNorm >= 0.25 && yNorm < 0.7 {
		sexGT = "XO/XY"
	}

	if len(sexGT) == 1 && yCopy == 0 {
		sexGT = "XO"
	}

	var gender string
	switch sexGT {
	case "XX":
		gender = "female"
	case "XY", "XO/XY":
		gender = "male"
	default:
		// Deal with unknowns depending on --force-male-female flag
		if forceMF {
			if yNorm < 0.00001 {
				gender = "female"
			} else {
				gender = "male"
			}
		} else {
			gender = "unknown"
		}
	}

	return &Sex{
		Gender:   gender,
		Genotype: sexGT,
		NormXEst: xNorm,
		NormYEst: yNorm,
	}
}

// Sex estimates the sex genotype for the given index
func (i *Index) Sex(ploidy uint, forceMF bool) (*Sex, error) {
	rawBins, err := i.Sizes(true)
	if err != nil {
		return nil, err
	}

	autoMedianSize, err := autosomalMedian(rawBins.Chroms, rawBins.RawSizes)
	if err != nil {
		return nil, err
	}

	var (
		xNorm float64
		yNorm float64
	)

	// divide per chromosome median byte size by the autosome
	// median byte size to get approx. copy number for chrom.
	for idx, refSizes := range rawBins.RawSizes {
		if rawBins.Chroms[idx] == "X" || rawBins.Chroms[idx] == "chrX" {
			refMedianSize, err := chromMedianOrZero(refSizes)
			if err != nil {
				return nil, fmt.Errorf("chromosome %s: %w", rawBins.Chroms[idx], err)
			}
			xNorm = float64(ploidy) * (refMedianSize / autoMedianSize)
		}
		if rawBins.Chroms[idx] == "Y" || rawBins.Chroms[idx] == "chrY" {
			refMedianSize, err := chromMedianOrZero(refSizes)
			if err != nil {
				return nil, fmt.Errorf("chromosome %s: %w", rawBins.Chroms[idx], err)
			}
			yNorm = float64(ploidy) * (refMedianSize / autoMedianSize)
		}
	}

	result := estimateSex(xNorm, yNorm, forceMF)
	result.Sample = i.Sample
	return result, nil
}

// Sizes estimates the raw/normalized per bin sizes for the given index
func (i *Index) Sizes(rawSize bool) (*Sizes, error) {
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
		for binNum, binSize := range refBins {
			if binSize < 0 {
				return nil, fmt.Errorf("%w: negative bin size for ref %d bin %d", errMalformedIndex, refID, binNum)
			}
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
		return data, nil
	}

	if err := data.Normalize(); err != nil {
		return nil, err
	}
	return data, nil
}

// NewIndex builds a new index given the path to index file and optionally path to reference fasta index.
func NewIndex(idxPath, faiPath string) (*Index, error) {
	return NewIndexWithOptions(idxPath, IndexOptions{FAIPath: faiPath})
}

// NewIndexWithOptions builds a new index with explicit reference handling options.
func NewIndexWithOptions(idxPath string, opts IndexOptions) (*Index, error) {
	opts = opts.withDefaults()
	sample := stripKnownSuffixes(idxPath)
	records, _, err := readReferenceRecords(idxPath, opts)
	if err != nil {
		return nil, err
	}

	build := opts.ReferenceBuild
	if build == ReferenceBuildAuto {
		build, err = detectReferenceBuild(records)
		if err != nil {
			return nil, err
		}
	}

	bins, err := ReadBins(idxPath, build.zeroMaskKey())
	if err != nil {
		return nil, err
	}

	return &Index{
		Bins:           bins,
		RefMap:         records.refMap(),
		RefLengths:     records.refLengths(),
		ReferenceBuild: build,
		Sample:         sample,
	}, nil
}
