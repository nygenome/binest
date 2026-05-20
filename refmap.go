package binest

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"

	"strings"

	"github.com/biogo/biogo/io/seqio/fai"
	"github.com/biogo/hts/bam"
)

// RefMap holds a mapping of ref idx to ref name
type RefMap map[int]string

// GenomeBuild detects the genome build for the reference
func (r *RefMap) GenomeBuild() string {
	for _, refName := range *r {
		if excludeChroms.MatchString(refName) {
			continue
		}
		if strings.HasSuffix(refName, "chr") {
			return "b38"
		}
	}
	return "b37"
}

// ReadRefMap reads a refmap given the index path and optionally path to fasta index
// For BAI indexes - it first looks for the matching bam for the BAI, falling back to FAI
// For TBI indexes - it always looks for a FAI index to build the refmap.
func ReadRefMap(idxPath, faiPath string) (*RefMap, error) {
	switch DetectIndexKind(idxPath) {
	case BaiIndex:
		return baiRefMap(idxPath, faiPath)
	case TbiIndex:
		return faiRefMap(faiPath)
	}
	return nil, unsupportedIndexError(idxPath)
}

func baiRefMap(idxPath, faiPath string) (*RefMap, error) {
	if faiPath != "" {
		return faiRefMap(faiPath)
	}

	bamPath, err := getBamPath(idxPath)
	if err != nil {
		return nil, fmt.Errorf("no bam/fai file provided to build refmap for index: %s: %w", idxPath, err)
	}

	bamFh, err := os.Open(bamPath)
	if err != nil {
		return nil, err
	}

	bamRdr, err := bam.NewReader(bamFh, runtime.GOMAXPROCS(0))
	if err != nil {
		if closeErr := bamFh.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}

	refMap := make(RefMap, len(bamRdr.Header().Refs()))
	for _, ref := range bamRdr.Header().Refs() {
		refMap[ref.ID()] = ref.Name()
	}

	if err := bamRdr.Close(); err != nil {
		if closeErr := bamFh.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	if err := bamFh.Close(); err != nil {
		return nil, err
	}
	return &refMap, nil
}

func faiRefMap(faiPath string) (*RefMap, error) {
	fh, err := os.Open(faiPath)
	if err != nil {
		return nil, err
	}

	faiIdx, err := fai.ReadFrom(bufio.NewReader(fh))
	closeErr := fh.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}

	faiRecords := make([]fai.Record, 0, len(faiIdx))
	for _, record := range faiIdx {
		faiRecords = append(faiRecords, record)
	}

	sort.Slice(faiRecords, func(i, j int) bool {
		return faiRecords[i].Start < faiRecords[j].Start
	})

	refMap := make(RefMap, len(faiRecords))
	for idx, record := range faiRecords {
		refMap[idx] = record.Name
	}

	return &refMap, nil
}

// getBamPath gets the BAM path given it's index
func getBamPath(idxPath string) (string, error) {
	prefix := idxPath[:len(idxPath)-4]

	if _, err := os.Stat(prefix); err == nil {
		return prefix, nil
	} else if _, err := os.Stat(prefix + ".bam"); err == nil {
		return prefix + ".bam", nil
	} else {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("could not find bam file for index: %s", idxPath)
		} else if os.IsPermission(err) {
			return "", fmt.Errorf("no permission to read bam file for index: %s", idxPath)
		} else {
			return "", fmt.Errorf("could not stat bam file for index: %s", idxPath)
		}
	}
}
