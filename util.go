package binest

import (
	"math"
	"sort"
)

// roundF64 returns the rounded value of a float64.
// It returns `precision` decimal places and rounds
// to next integer value at `threshold` mark.
func roundF64(value, threshold float64, precision int) (rounded float64) {
	scaling := math.Pow(10, float64(precision))
	digits := value * scaling
	_, frac := math.Modf(digits)
	if frac >= threshold {
		rounded = math.Ceil(digits)
	} else {
		rounded = math.Floor(digits)
	}
	rounded /= scaling
	return rounded
}

// medianI64 returns the median from []int64.
// recursively finds a non-zero median if possible.
func medianI64(arr []int64) (median float64) {
	lessFunc := func(i, j int) bool { return arr[i] < arr[j] }
	if !sort.SliceIsSorted(arr, lessFunc) {
		sort.Slice(arr, lessFunc)
	}

	arrLen := len(arr)
	if arrLen%2 == 0 {
		median = float64(arr[arrLen/2-1]+arr[arrLen/2+1]) / float64(2)
	} else {
		median = float64(arr[arrLen/2])
	}

	if median == 0 {
		currIdx := arrLen / 2
		for ; currIdx < arrLen && arr[currIdx] == 0; currIdx++ {
		}
		return medianI64(arr[currIdx:])
	}

	return median
}

// medianF64 returns the median from []float64.
// recursively finds a non-zero median if possible.
func medianF64(arr []float64) (median float64) {
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
func meanI64(arr []int64) (mean float64) {
	var sum int64
	for _, val := range arr {
		sum += val
	}
	mean = float64(sum) / float64(len(arr))
	return mean
}

// meanF64 returns the mean from []float64.
func meanF64(arr []float64) (mean float64) {
	var sum float64
	for _, val := range arr {
		sum += val
	}
	mean = sum / float64(len(arr))
	return mean
}
