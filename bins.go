package binest

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/gob"
	"os"
	"reflect"
	"unsafe"

	"github.com/biogo/hts/bam"
	"github.com/biogo/hts/bgzf"
	"github.com/biogo/hts/tabix"
	"gopkg.in/src-d/go-errors.v1"

	"git.nygenome.org/rmusunuri/binest/internal"
)

// ZeroBins holds all the bins which are predominantly zeros all the time
// calculated from 1000 known male and female samples for both b37 and b38
// Any bin where > 990 out of the 1000 samples are of zero size in both
// male and female samples are used and embedded inside the binary.
type ZeroBins map[string]map[int]map[int]bool

var zeros ZeroBins

//go:embed resources/refbins.zeros
var refbinsZeros []byte

func init() {
	dec := gob.NewDecoder(bytes.NewReader(refbinsZeros))
	if err := dec.Decode(&zeros); err != nil {
		panic(err)
	}
}

// errUnsupprtedIndex is returned when trying to read an unknown/unsupported index
var errUnsupprtedIndex = errors.NewKind("unknown/unsupported index: %s")

// Bins holds the raw byte sizes for each bin the the index
// 1st dim - ref idx, 2 dim - windows within that ref idx
type Bins [][]int64

// ReadBins reads the given index and returns its bin data
func ReadBins(idxPath, build string) (*Bins, error) {
	switch idxKind := DetectIndexKind(idxPath); idxKind {
	case BaiIndex:
		if refIdxs, err := baiRefIdxs(idxPath); err != nil {
			return nil, err
		} else {
			return binSizes(refIdxs, build), nil
		}
	case TbiIndex:
		if refIdxs, err := tbiRefIdxs(idxPath); err != nil {
			return nil, err
		} else {
			return binSizes(refIdxs, build), nil
		}
	}
	return nil, errUnsupprtedIndex.New(idxPath)
}

// binSizes returns sizes of all bins from the refIdxs
func binSizes(refIdxs []internal.RefIndex, build string) *Bins {
	bins := make(Bins, len(refIdxs))

	for refNum, refIdx := range refIdxs {
		if len(refIdx.Intervals) < 2 {
			bins[refNum] = []int64{}
			continue
		}

		bins[refNum] = make([]int64, len(refIdx.Intervals)-1)
		for binNum, intervalEnd := range refIdx.Intervals[1:] {
			if isZero, found := zeros[build][refNum][binNum]; found && isZero {
				continue
			}

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
