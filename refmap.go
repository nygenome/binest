package binest

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"sort"

	"github.com/biogo/biogo/io/seqio/fai"
	"github.com/biogo/hts/bam"
	"github.com/biogo/hts/bgzf"
	"github.com/biogo/hts/tabix"
)

// RefMap holds a mapping of ref idx to ref name
type RefMap map[int]string

// RefLengths holds a mapping of ref idx to ref length.
type RefLengths map[int]int

var errBAMNotFound = errors.New("matching BAM not found")

// ReadRefMap reads a refmap given the index path and optionally path to fasta index
// For BAI indexes - it first looks for the matching bam for the BAI, falling back to FAI
// For TBI indexes - it always looks for a FAI index to build the refmap.
func ReadRefMap(idxPath, faiPath string) (*RefMap, error) {
	records, _, err := readReferenceRecords(idxPath, IndexOptions{FAIPath: faiPath})
	if err != nil {
		return nil, err
	}
	return records.refMap(), nil
}

// ValidateIndexReferences compares available index reference names with a supplied FAI.
func ValidateIndexReferences(idxPath, faiPath string) (*ReferenceValidationReport, error) {
	if faiPath == "" {
		return nil, nil
	}
	_, report, err := readReferenceRecords(idxPath, IndexOptions{
		FAIPath:             faiPath,
		ReferenceValidation: ReferenceValidationAllowMismatch,
	})
	return report, err
}

func readReferenceRecords(idxPath string, opts IndexOptions) (referenceRecords, *ReferenceValidationReport, error) {
	opts = opts.withDefaults()
	switch DetectIndexKind(idxPath) {
	case BaiIndex:
		return baiReferenceRecords(idxPath, opts)
	case TbiIndex:
		return tbiReferenceRecords(idxPath, opts)
	}
	return nil, nil, unsupportedIndexError(idxPath)
}

func baiReferenceRecords(idxPath string, opts IndexOptions) (referenceRecords, *ReferenceValidationReport, error) {
	if opts.FAIPath == "" {
		records, found, err := bamReferenceRecordsForIndex(idxPath)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			return nil, nil, fmt.Errorf("no bam/fai file provided to build refmap for index: %s: could not find matching bam file", idxPath)
		}
		return records, nil, nil
	}

	faiRecords, err := faiReferenceRecords(opts.FAIPath)
	if err != nil {
		return nil, nil, err
	}
	bamRecords, found, err := bamReferenceRecordsForIndex(idxPath)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return faiRecords, nil, nil
	}
	report := compareReferenceRecords(idxPath, opts.FAIPath, "BAM header", bamRecords, faiRecords, true)
	if report.HasMismatch() && opts.ReferenceValidation != ReferenceValidationAllowMismatch {
		return nil, report, report
	}
	return faiRecords, report, nil
}

func tbiReferenceRecords(idxPath string, opts IndexOptions) (referenceRecords, *ReferenceValidationReport, error) {
	if opts.FAIPath == "" {
		return nil, nil, fmt.Errorf("no fai file provided to build refmap for tabix index: %s", idxPath)
	}
	faiRecords, err := faiReferenceRecords(opts.FAIPath)
	if err != nil {
		return nil, nil, err
	}
	tbiRecords, err := tabixReferenceRecords(idxPath)
	if err != nil {
		return nil, nil, err
	}
	report := compareReferenceRecords(idxPath, opts.FAIPath, "tabix index", tbiRecords, faiRecords, false)
	if report.HasMismatch() && opts.ReferenceValidation != ReferenceValidationAllowMismatch {
		return nil, report, report
	}
	return faiRecords, report, nil
}

func tabixReferenceRecords(idxPath string) (referenceRecords, error) {
	fh, err := os.Open(idxPath)
	if err != nil {
		return nil, fmt.Errorf("open TBI %q: %w", idxPath, err)
	}

	tbxRdr, err := bgzf.NewReader(bufio.NewReader(fh), runtime.GOMAXPROCS(0))
	if err != nil {
		if closeErr := fh.Close(); closeErr != nil {
			return nil, errors.Join(
				fmt.Errorf("open BGZF reader for TBI %q: %w", idxPath, err),
				fmt.Errorf("close TBI %q: %w", idxPath, closeErr),
			)
		}
		return nil, fmt.Errorf("open BGZF reader for TBI %q: %w", idxPath, err)
	}

	idx, readErr := tabix.ReadFrom(tbxRdr)
	tbxCloseErr := tbxRdr.Close()
	fhCloseErr := fh.Close()
	if readErr != nil {
		return nil, errors.Join(
			fmt.Errorf("read TBI %q: %w", idxPath, readErr),
			closePathError("close BGZF reader for TBI", idxPath, tbxCloseErr),
			closePathError("close TBI", idxPath, fhCloseErr),
		)
	}
	if tbxCloseErr != nil || fhCloseErr != nil {
		return nil, errors.Join(
			closePathError("close BGZF reader for TBI", idxPath, tbxCloseErr),
			closePathError("close TBI", idxPath, fhCloseErr),
		)
	}

	names := idx.Names()
	records := make(referenceRecords, 0, len(names))
	for id, name := range names {
		records = append(records, referenceRecord{ID: id, Name: name})
	}
	return records, nil
}

func bamReferenceRecordsForIndex(idxPath string) (referenceRecords, bool, error) {
	bamPath, err := getBamPath(idxPath)
	if err != nil {
		if errors.Is(err, errBAMNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if bamPath == "" {
		return nil, false, nil
	}
	bamFh, err := os.Open(bamPath)
	if err != nil {
		return nil, true, fmt.Errorf("open BAM %q for index %q: %w", bamPath, idxPath, err)
	}

	bamRdr, err := bam.NewReader(bamFh, runtime.GOMAXPROCS(0))
	if err != nil {
		if closeErr := bamFh.Close(); closeErr != nil {
			return nil, true, errors.Join(
				fmt.Errorf("read BAM header %q for index %q: %w", bamPath, idxPath, err),
				fmt.Errorf("close BAM %q: %w", bamPath, closeErr),
			)
		}
		return nil, true, fmt.Errorf("read BAM header %q for index %q: %w", bamPath, idxPath, err)
	}

	records := make(referenceRecords, 0, len(bamRdr.Header().Refs()))
	for _, ref := range bamRdr.Header().Refs() {
		records = append(records, referenceRecord{ID: ref.ID(), Name: ref.Name(), Length: ref.Len()})
	}

	if err := bamRdr.Close(); err != nil {
		if closeErr := bamFh.Close(); closeErr != nil {
			return nil, true, errors.Join(
				fmt.Errorf("close BAM reader %q: %w", bamPath, err),
				fmt.Errorf("close BAM %q: %w", bamPath, closeErr),
			)
		}
		return nil, true, fmt.Errorf("close BAM reader %q: %w", bamPath, err)
	}
	if err := bamFh.Close(); err != nil {
		return nil, true, fmt.Errorf("close BAM %q: %w", bamPath, err)
	}
	return records, true, nil
}

func closePathError(op, path string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s %q: %w", op, path, err)
}

func faiReferenceRecords(faiPath string) (referenceRecords, error) {
	fh, err := os.Open(faiPath)
	if err != nil {
		return nil, fmt.Errorf("open FAI %q: %w", faiPath, err)
	}

	faiIdx, err := fai.ReadFrom(bufio.NewReader(fh))
	closeErr := fh.Close()
	if err != nil {
		if closeErr != nil {
			return nil, errors.Join(
				fmt.Errorf("read FAI %q: %w", faiPath, err),
				fmt.Errorf("close FAI %q: %w", faiPath, closeErr),
			)
		}
		return nil, fmt.Errorf("read FAI %q: %w", faiPath, err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close FAI %q: %w", faiPath, closeErr)
	}

	faiRecords := make([]fai.Record, 0, len(faiIdx))
	for _, record := range faiIdx {
		faiRecords = append(faiRecords, record)
	}

	sort.Slice(faiRecords, func(i, j int) bool {
		return faiRecords[i].Start < faiRecords[j].Start
	})

	records := make(referenceRecords, 0, len(faiRecords))
	for idx, record := range faiRecords {
		records = append(records, referenceRecord{ID: idx, Name: record.Name, Length: record.Length})
	}

	return records, nil
}

// getBamPath gets the BAM path given it's index
func getBamPath(idxPath string) (string, error) {
	prefix := idxPath[:len(idxPath)-4]

	if _, err := os.Stat(prefix); err == nil {
		return prefix, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("stat BAM candidate %q for index %q: %w", prefix, idxPath, err)
	}

	bamPath := prefix + ".bam"
	if _, err := os.Stat(bamPath); err == nil {
		return prefix + ".bam", nil
	} else if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%w for index: %s", errBAMNotFound, idxPath)
	} else {
		return "", fmt.Errorf("stat BAM candidate %q for index %q: %w", bamPath, idxPath, err)
	}
}
