package binest

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

var errInvalidMedianInput = errors.New("invalid median input")

// roundChromSize returns the rounded copy number from the nomalized chrom size.
// It returns `precision` decimal places and rounds to next integer value
// at `threshold` mark. values with frac >= 0.7 are ceiled else floored.
func roundChromSize(normSize float64) (copyNum uint8) {
	_, frac := math.Modf(normSize)
	switch {
	case frac >= 0.7:
		copyNum = uint8(math.Ceil(normSize))
	default:
		copyNum = uint8(math.Floor(normSize))
	}
	return copyNum
}

// medianI64 returns the median from []int64.
// recursively finds a non-zero median if possible.
func medianI64(arr []int64) (float64, error) {
	if len(arr) <= 2 {
		return meanI64(arr)
	}

	lessFunc := func(i, j int) bool { return arr[i] < arr[j] }
	if !sort.SliceIsSorted(arr, lessFunc) {
		sort.Slice(arr, lessFunc)
	}

	var median float64
	arrLen := len(arr)
	if arrLen%2 == 0 {
		median = (float64(arr[arrLen/2-1]) + float64(arr[arrLen/2])) / 2
	} else {
		median = float64(arr[arrLen/2])
	}

	if median == 0 {
		currIdx := arrLen / 2
		for ; currIdx < arrLen && arr[currIdx] == 0; currIdx++ {
		}
		if currIdx == arrLen {
			return 0, nil
		}
		return medianI64(arr[currIdx:])
	}

	return median, nil
}

// meanI64 returns the mean from []int64.
func meanI64(arr []int64) (float64, error) {
	if len(arr) == 0 {
		return 0, fmt.Errorf("%w: empty slice", errInvalidMedianInput)
	}
	var sum int64
	for _, val := range arr {
		sum += val
	}
	return float64(sum) / float64(len(arr)), nil
}

// stripKnownSuffixes strips known index suffixes to get sample name
func stripKnownSuffixes(path string) string {
	suffixes := []string{".final.bam.bai", ".final.bai", ".bam.bai", ".bai", ".vcf.gz.tbi"}
	out := filepath.Base(path)
	for _, suff := range suffixes {
		out = strings.TrimSuffix(out, suff)
	}
	return out
}
