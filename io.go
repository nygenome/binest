package binest

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unsafe"

	"github.com/biogo/biogo/io/seqio/fai"
	"github.com/biogo/hts/bam"
	"github.com/biogo/hts/bgzf"
	"github.com/biogo/hts/tabix"
	"github.com/pkg/errors"

	"github.com/omicsnut/binest/internal"
)

// IndexType is the type of the index
type IndexType uint8

// Constants holding the supported index types
const (
	UNK IndexType = iota
	BAI
	TBI
)

// vOffset returns the virtual file offset from a BGZF offset.
func vOffset(o bgzf.Offset) int64 {
	return o.File<<16 | int64(o.Block)
}

// detectIndex returns the sample name and indexType from index path
func detectIndex(idxPath string) (string, IndexType) {
	sampleName := strings.SplitN(filepath.Base(idxPath), ".", 2)[0]

	switch filepath.Ext(idxPath) {
	case ".bai":
		return sampleName, BAI
	case ".tbi":
		return sampleName, TBI
	default:
		return sampleName, UNK
	}
}

// GetRefMap returns map[uint32]string from the reference FAI path
func GetRefMap(faiPath string) (map[uint32]string, error) {
	fh, err := os.Open(faiPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Error opening FAI: %s", faiPath)
	}
	defer fh.Close()

	faiIdx, err := fai.ReadFrom(bufio.NewReader(fh))
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading FAI: %s", faiPath)
	}

	faiRecords := make([]fai.Record, 0, len(faiIdx))
	for _, record := range faiIdx {
		faiRecords = append(faiRecords, record)
	}

	sort.Slice(faiRecords, func(i, j int) bool {
		return faiRecords[i].Start < faiRecords[j].Start
	})

	refMap := make(map[uint32]string, len(faiRecords))
	for idx, record := range faiRecords {
		refMap[uint32(idx)] = record.Name
	}

	return refMap, nil
}

// baiRefIdxs returns the slice of reference indexes from a BAI index
func baiRefIdxs(idxPath string) ([]internal.RefIndex, error) {
	fh, err := os.Open(idxPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Error open BAI: %s", idxPath)
	}
	defer fh.Close()

	idx, err := bam.ReadIndex(bufio.NewReader(fh))
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading BAI: %s", idxPath)
	}

	idxRefs := reflect.ValueOf(*idx).FieldByName("idx").FieldByName("Refs")
	idxRefsPtr := unsafe.Pointer(idxRefs.Pointer())
	refIdxs := (*(*[1 << 29]internal.RefIndex)(idxRefsPtr))[:idxRefs.Len()]

	return refIdxs, nil
}

// tbiRefIdxs returns the slice of reference indexes from a TABIX index
func tbiRefIdxs(idxPath string) ([]internal.RefIndex, error) {
	fh, err := os.Open(idxPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Error opening TBI: %s", idxPath)
	}
	defer fh.Close()

	tbxRdr, err := bgzf.NewReader(bufio.NewReader(fh), 2)
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading TBI: %s", idxPath)
	}

	idx, err := tabix.ReadFrom(tbxRdr)
	if err != nil {
		return nil, errors.Wrapf(err, "Error reading TBI: %s", idxPath)
	}

	idxRefs := reflect.ValueOf(*idx).FieldByName("idx").FieldByName("Refs")
	idxRefsPtr := unsafe.Pointer(idxRefs.Pointer())
	refIdxs := (*(*[1 << 29]internal.RefIndex)(idxRefsPtr))[:idxRefs.Len()]

	return refIdxs, nil
}

// binSizes returns sizes of all bins from the refIdxs
func binSizes(refIdxs []internal.RefIndex) [][]int64 {
	bins := make([][]int64, len(refIdxs))

	for refNum, refIdx := range refIdxs {
		if len(refIdx.Intervals) < 2 {
			bins[refNum] = make([]int64, 0)
			continue
		}

		bins[refNum] = make([]int64, len(refIdx.Intervals)-1)
		for binNum, intervalEnd := range refIdx.Intervals[1:] {
			bins[refNum][binNum] = vOffset(intervalEnd) - vOffset(refIdx.Intervals[binNum])
		}

		refIdx.Bins, refIdx.Intervals = nil, nil
	}

	return bins
}
