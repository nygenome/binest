package binest

import (
	"sort"

	"github.com/biogo/hts/bgzf"
)

// vOffset returns the BAM virtual offset from BGZF offset
func vOffset(o bgzf.Offset) int64 {
	return o.File<<16 | int64(o.Block)
}

// Median gets the median for a slice of bin sizes
func Median(input []int64) float64 {
	arrLen := len(input)
	sort.Slice(input, func(i, j int) bool { return input[i] < input[j] })

	var median float64
	if arrLen%2 == 0 {
		median = float64(input[arrLen/2-1]+input[arrLen/2+1]) / float64(2)
	} else {
		median = float64(input[arrLen/2])
	}

	if median == 0 {
		curIdx := arrLen / 2
		for ; curIdx < arrLen && input[curIdx] == 0; curIdx++ {
		}
		return Median(input[curIdx:])
	}

	return median
}
