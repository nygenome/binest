package binest

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"reflect"
	"unsafe"

	"github.com/biogo/hts/bam"
	"github.com/biogo/hts/bgzf"
	"github.com/biogo/hts/tabix"

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

var (
	// errUnsupportedIndex is returned when trying to read an unknown/unsupported index.
	errUnsupportedIndex = errors.New("unknown/unsupported index")
	errMalformedIndex   = errors.New("malformed index")
)

func unsupportedIndexError(idxPath string) error {
	return fmt.Errorf("%w: %s", errUnsupportedIndex, idxPath)
}

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
			return binSizes(refIdxs, build)
		}
	case TbiIndex:
		if refIdxs, err := tbiRefIdxs(idxPath); err != nil {
			return nil, err
		} else {
			return binSizes(refIdxs, build)
		}
	}
	return nil, unsupportedIndexError(idxPath)
}

// binSizes returns sizes of all bins from the refIdxs.
func binSizes(refIdxs []internal.RefIndex, build string) (*Bins, error) {
	bins := make(Bins, len(refIdxs))

	for refNum, refIdx := range refIdxs {
		if len(refIdx.Intervals) < 2 {
			bins[refNum] = []int64{}
			continue
		}

		bins[refNum] = make([]int64, len(refIdx.Intervals)-1)
		for binNum, intervalEnd := range refIdx.Intervals[1:] {
			intervalStart := refIdx.Intervals[binNum]
			startOffset := vOffset(intervalStart)
			endOffset := vOffset(intervalEnd)
			if endOffset < startOffset {
				return nil, fmt.Errorf("%w: non-monotonic interval offsets for ref %d bin %d", errMalformedIndex, refNum, binNum)
			}

			if isZero, found := zeros[build][refNum][binNum]; found && isZero {
				continue
			}

			bins[refNum][binNum] = endOffset - startOffset
		}

		refIdx.Bins, refIdx.Intervals = nil, nil
	}

	return &bins, nil
}

// vOffset returns the virtual file offset from a BGZF offset.
func vOffset(o bgzf.Offset) int64 {
	return o.File<<16 | int64(o.Block)
}

// baiRefIdxs returns the slice of reference indexes from a BAI index
func baiRefIdxs(idxPath string) ([]internal.RefIndex, error) {
	fh, err := os.Open(idxPath)
	if err != nil {
		return nil, fmt.Errorf("open BAI %q: %w", idxPath, err)
	}

	idx, err := bam.ReadIndex(bufio.NewReader(fh))
	closeErr := fh.Close()
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("read BAI %q: %w", idxPath, err),
			closePathError("close BAI", idxPath, closeErr),
		)
	}
	if closeErr != nil {
		return nil, closePathError("close BAI", idxPath, closeErr)
	}

	return biogoRefIdxs(idx, "BAI")
}

// tbiRefIdxs returns the slice of reference indexes from a TABIX index
func tbiRefIdxs(idxPath string) ([]internal.RefIndex, error) {
	fh, err := os.Open(idxPath)
	if err != nil {
		return nil, fmt.Errorf("open TBI %q: %w", idxPath, err)
	}

	tbxRdr, err := bgzf.NewReader(bufio.NewReader(fh), 2)
	if err != nil {
		if closeErr := fh.Close(); closeErr != nil {
			return nil, errors.Join(
				fmt.Errorf("open BGZF reader for TBI %q: %w", idxPath, err),
				closePathError("close TBI", idxPath, closeErr),
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

	return biogoRefIdxs(idx, "TBI")
}

func biogoRefIdxs(index any, source string) ([]internal.RefIndex, error) {
	refs, err := biogoRefIdxValue(index, source)
	if err != nil {
		return nil, err
	}
	if refs.Len() == 0 {
		return []internal.RefIndex{}, nil
	}
	idxRefsPtr := unsafe.Pointer(refs.Pointer())
	refIdxs := (*(*[1 << 29]internal.RefIndex)(idxRefsPtr))[:refs.Len():refs.Len()]
	return append([]internal.RefIndex(nil), refIdxs...), nil
}

func biogoRefIdxValue(index any, source string) (reflect.Value, error) {
	v := reflect.ValueOf(index)
	if !v.IsValid() {
		return reflect.Value{}, fmt.Errorf("%s index internals unavailable: nil index", source)
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}, fmt.Errorf("%s index internals unavailable: nil index", source)
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("%s index internals unavailable: got %s, want struct", source, v.Kind())
	}
	idx := v.FieldByName("idx")
	if !idx.IsValid() || idx.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("%s index internals changed: missing struct field idx", source)
	}
	refs := idx.FieldByName("Refs")
	if !refs.IsValid() || refs.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("%s index internals changed: missing slice field idx.Refs", source)
	}
	refType := refs.Type().Elem()
	if refType.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("%s index internals changed: idx.Refs element is %s, want struct", source, refType.Kind())
	}
	if refType.NumField() < 3 {
		return reflect.Value{}, fmt.Errorf("%s index internals changed: idx.Refs element layout is incompatible", source)
	}
	for _, name := range []string{"Bins", "Stats", "Intervals"} {
		if _, ok := refType.FieldByName(name); !ok {
			return reflect.Value{}, fmt.Errorf("%s index internals changed: idx.Refs element missing %s", source, name)
		}
	}
	if refType.Size() != reflect.TypeOf(internal.RefIndex{}).Size() {
		return reflect.Value{}, fmt.Errorf("%s index internals changed: RefIndex size mismatch", source)
	}
	return refs, nil
}
