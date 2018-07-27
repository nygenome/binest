package binest

import (
	"bufio"
	"os"
	"reflect"
	"unsafe"

	"github.com/biogo/hts/bam"
	"github.com/biogo/hts/bgzf"
	"github.com/biogo/hts/tabix"
	"gopkg.in/src-d/go-errors.v1"

	"git.nygenome.org/rmusunuri/binest/internal"
)

// errUnsupprtedIndex is returned when trying to read an unknown/unsupported index
var errUnsupprtedIndex = errors.NewKind("unknown/unsupported index: %s")

// Bins holds the raw byte sizes for each bin the the index
// 1st dim - ref idx, 2 dim - windows within that ref idx
type Bins [][]int64

// ReadBins reads the given index and returns its bin data
func ReadBins(idxPath string) (*Bins, error) {
	switch idxKind := DetectIndexKind(idxPath); idxKind {
	case BaiIndex:
		if refIdxs, err := baiRefIdxs(idxPath); err != nil {
			return nil, err
		} else {
			return binSizes(refIdxs), nil
		}
	case TbiIndex:
		if refIdxs, err := tbiRefIdxs(idxPath); err != nil {
			return nil, err
		} else {
			return binSizes(refIdxs), nil
		}
	}
	return nil, errUnsupprtedIndex.New(idxPath)
}

// binSizes returns sizes of all bins from the refIdxs
func binSizes(refIdxs []internal.RefIndex) *Bins {
	bins := make(Bins, len(refIdxs))

	for refNum, refIdx := range refIdxs {
		if len(refIdx.Intervals) < 2 {
			bins[refNum] = []int64{}
			continue
		}

		bins[refNum] = make([]int64, len(refIdx.Intervals)-1)
		for binNum, intervalEnd := range refIdx.Intervals[1:] {
			bins[refNum][binNum] = vOffset(intervalEnd) - vOffset(refIdx.Intervals[binNum])
		}

		refIdx.Bins, refIdx.Intervals = nil, nil
	}

	return &bins
}

// vOffset returns the virtual file offset from a BGZF offset.
func vOffset(o bgzf.Offset) int64 {
	return o.File<<16 | int64(o.Block)
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
