package binest

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"unsafe"

	"github.com/biogo/biogo/io/seqio/fai"
	"github.com/biogo/hts/bam"
	"github.com/biogo/hts/bgzf"
	"github.com/biogo/hts/tabix"

	"git.nygenome.org/rmusunuri/binest/internal"
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
	suffixes := []string{".final.bam.bai", ".final.bai", ".bam.bai", ".bai", ".vcf.gz.tbi"}
	basename := filepath.Base(idxPath)

	// recursively trim possible suffixes to get sample name
	sampleName := strings.TrimSuffix(basename, suffixes[0])
	for _, suff := range suffixes[1:] {
		sampleName = strings.TrimSuffix(sampleName, suff)
	}

	switch filepath.Ext(idxPath) {
	case ".bai":
		return sampleName, BAI
	case ".tbi":
		return sampleName, TBI
	default:
		return sampleName, UNK
	}
}

// getBamPath gets the BAM path given it's index
func getBamPath(idxPath string) (string, error) {
	prefix := idxPath[:len(idxPath)-4]

	if _, err := os.Stat(prefix); err == nil {
		return prefix, nil
	} else if _, err := os.Stat(prefix + ".bam"); err == nil {
		return prefix + ".bam", nil
	}

	return "", fmt.Errorf("couldn't find BAM file for %s", idxPath)
}

// getRefMapBamIdx gets the reference index map from BAM header
func getRefMapBamIdx(bamIdxPath string) (map[uint32]string, error) {
	bamPath, err := getBamPath(bamIdxPath)

	if err != nil {
		return nil, err
	}

	bamFh, err := os.Open(bamPath)
	if err != nil {
		return nil, err
	}

	bamRdr, err := bam.NewReader(bufio.NewReader(bamFh), runtime.GOMAXPROCS(0))
	if err != nil {
		return nil, err
	}

	defer bamFh.Close()
	defer bamRdr.Close()

	refMap := make(map[uint32]string, len(bamRdr.Header().Refs()))
	for _, ref := range bamRdr.Header().Refs() {
		refMap[uint32(ref.ID())] = ref.Name()
	}
	return refMap, nil
}

// getRefMapFaiIdx gets the reference index map from FAI index
func getRefMapFaiIdx(faiIdxPath string) (map[uint32]string, error) {
	fh, err := os.Open(faiIdxPath)
	defer fh.Close()
	if err != nil {
		return nil, err
	}

	faiIdx, err := fai.ReadFrom(bufio.NewReader(fh))
	if err != nil {
		return nil, err
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

// GetRefMap returns map[uint32]string from the reference FAI path
func GetRefMap(faiPath string, idxPath string) (map[uint32]string, error) {
	// if not FAI index provided and working with a TABIX index, bail out immediately
	if faiPath == "" && !strings.HasSuffix(idxPath, ".bai") {
		return nil, errors.New("need FAI reference index for TABIX indexes")
	}

	// if no FAI index provided and working with a BAM index
	if faiPath == "" {
		return getRefMapBamIdx(idxPath)
	}

	// FAI index provided and working with either BAM/TABIX index
	return getRefMapFaiIdx(faiPath)
}

// baiRefIdxs returns the slice of reference indexes from a BAI index
func baiRefIdxs(idxPath string) ([]internal.RefIndex, error) {
	fh, err := os.Open(idxPath)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	idx, err := bam.ReadIndex(bufio.NewReader(fh))
	if err != nil {
		return nil, err
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
		return nil, err
	}
	defer fh.Close()

	tbxRdr, err := bgzf.NewReader(bufio.NewReader(fh), 2)
	if err != nil {
		return nil, err
	}

	idx, err := tabix.ReadFrom(tbxRdr)
	if err != nil {
		return nil, err
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
