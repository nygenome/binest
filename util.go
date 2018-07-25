package binest

import (
	"math"
	"sort"
)

// roundChromSize returns the rounded copy number from the nomalized chrom size.
// It returns `precision` decimal places and rounds to next integer value
// at `threshold` mark. values with frac >= 0.7 are ceiled else floored.
func roundChromSize(normSize float64) (copyNum uint32) {
	_, frac := math.Modf(normSize)
	switch {
	case frac >= 0.7:
		copyNum = uint32(math.Ceil(normSize))
	default:
		copyNum = uint32(math.Floor(normSize))
	}
	return copyNum
}

// medianI64 returns the median from []int64.
// recursively finds a non-zero median if possible.
//func medianI64(arr []int64) (median float64) {
//	if len(arr) <= 2 {
//		return meanI64(arr)
//	}
//
//	lessFunc := func(i, j int) bool { return arr[i] < arr[j] }
//	if !sort.SliceIsSorted(arr, lessFunc) {
//		sort.Slice(arr, lessFunc)
//	}
//
//	arrLen := len(arr)
//	if arrLen%2 == 0 {
//		median = float64(arr[arrLen/2-1]+arr[arrLen/2+1]) / float64(2)
//	} else {
//		median = float64(arr[arrLen/2])
//	}
//
//	if median == 0 {
//		currIdx := arrLen / 2
//		for ; currIdx < arrLen && arr[currIdx] == 0; currIdx++ {
//		}
//		return medianI64(arr[currIdx:])
//	}
//
//	return median
//}

// medianF64 returns the median from []float64.
// recursively finds a non-zero median if possible.
func medianF64(arr []float64) (median float64) {
	if len(arr) <= 2 {
		return meanF64(arr)
	}

	lessFunc := func(i, j int) bool { return arr[i] < arr[j] }
	if !sort.SliceIsSorted(arr, lessFunc) {
		sort.Slice(arr, lessFunc)
	}

	arrLen := len(arr)
	if arrLen%2 == 0 {
		median = (arr[arrLen/2-1] + arr[arrLen/2+1]) / float64(2)
	} else {
		median = arr[arrLen/2]
	}

	if median == 0 {
		currIdx := arrLen / 2
		for ; currIdx < arrLen && arr[currIdx] == 0; currIdx++ {
		}
		return medianF64(arr[currIdx:])
	}

	return median
}

// meanI64 returns the mean from []int64.
//func meanI64(arr []int64) (mean float64) {
//	var sum int64
//	for _, val := range arr {
//		sum += val
//	}
//	mean = float64(sum) / float64(len(arr))
//	return mean
//}

// meanF64 returns the mean from []float64.
func meanF64(arr []float64) (mean float64) {
	var sum float64
	for _, val := range arr {
		sum += val
	}
	mean = sum / float64(len(arr))
	return mean
}
