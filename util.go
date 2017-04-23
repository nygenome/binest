package binest

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/biogo/hts/bgzf"
	"github.com/biogo/store/interval"
	"github.com/brentp/xopen"
)

// bedRecord is a regex which will match chrom:start-end and chrom\tstart\tend
var bedRecord = regexp.MustCompile("(.+?)[:\t](\\d+)([\\-\t])(\\d+).*?")

// ParseRegion reads a bed record from a line
func ParseRegion(line []byte) (string, int, int) {
	parsed := bedRecord.FindSubmatch(line)
	if len(parsed) != 5 {
		panic(fmt.Errorf("Couldn't parse genomic region from bed line - %s", string(line)))
	}
	chrom, start, isep, end := parsed[1], parsed[2], parsed[3], parsed[4]
	sChrom := string(chrom)
	intStart, err := strconv.Atoi(string(start))
	CheckError(err)

	if bytes.Equal(isep, []byte{'-'}) {
		intStart--
	}

	if intStart < 0 {
		intStart = 0
	}

	intEnd, err := strconv.Atoi(string(end))
	CheckError(err)

	return sChrom, intStart, intEnd
}

// CheckError checks for error and panics if present
func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

// VOffset returns a virtual offset from BGZF offset
func VOffset(o bgzf.Offset) int64 {
	return o.File<<16 | int64(o.Block)
}

// BgzfOffset returns a BGZF offset from virtual offset
func BgzfOffset(v int64) bgzf.Offset {
	start := v >> 16
	return bgzf.Offset{
		File:  start,
		Block: uint16(v ^ (start << 16)),
	}
}

// MedianInt64 gets the median for a slice of int64's
func MedianInt64(input []int64) float64 {
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
		return MedianInt64(input[curIdx:])
	}

	return median
}

// MedianFloat64 gets the median for a slice of float64's
func MedianFloat64(input []float64) float64 {
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
		return MedianFloat64(input[curIdx:])
	}

	return median
}

// MeanFloat64 gets the mean for a slice of float64's
func MeanFloat64(input []float64) float64 {
	var sum float64
	for _, val := range input {
		sum += val
	}
	return sum / float64(len(input))
}

// ShuffleChunks shuffles BGZF chunks using the fisher yates method
func ShuffleChunks(c []bgzf.Chunk) {
	for i := range c {
		j := rand.Intn(i + 1)
		c[i], c[j] = c[j], c[i]
	}
}

// HasStdin checks if data can be read from stdin
func HasStdin() bool {
	stat, err := os.Stdin.Stat()
	CheckError(err)
	if stat.Mode()&os.ModeCharDevice == 0 {
		return true
	}
	return false
}

// StreamLines reads lines from file handle and puts them in the channel
func StreamLines(fd *os.File, res chan<- string) {
	bufScnr := bufio.NewScanner(fd)
	for bufScnr.Scan() {
		res <- bufScnr.Text()
	}
	CheckError(bufScnr.Err())
}

// ReadBed takes a bed file and returns a map of int trees for overlap testing
func ReadBed(bedPath string, chromToID map[string]int) map[int]*interval.IntTree {
	rdr, err := xopen.Ropen(bedPath)
	CheckError(err)

	tree := make(map[int]*interval.IntTree)
	bufRdr := bufio.NewReader(rdr)

	for {
		line, err := bufRdr.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		CheckError(err)
		chrom, start, end := ParseRegion(line)

		if refID, ok := chromToID[chrom]; ok {
			if _, ok := tree[refID]; !ok {
				tree[refID] = &interval.IntTree{}
			}
			tree[refID].Insert(&RefBlock{RefID: refID, Start: start, End: end}, false)
		} else {
			panic(fmt.Errorf("RefID for %s not found", chrom))
		}
	}

	return tree
}

// Round rounds a float64 value at `roundOn` decimal point to `places`
func Round(val float64, roundOn float64, places int) float64 {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	return round / pow
}

// ReadIndex reads the index from bampath *.bam.bai and *.bai before failing
func ReadIndex(b string) *os.File {
	var bamIdxFh *os.File

	if _, err := os.Stat(fmt.Sprintf("%s.bai", b)); err == nil {
		bamIdxFh, err = os.Open(fmt.Sprintf("%s.bai", b))
		CheckError(err)
	} else if _, err = os.Stat(b[:len(b)-4] + ".bai"); err == nil {
		bamIdxFh, err = os.Open(b[:len(b)-4] + ".bai")
		CheckError(err)
	} else {
		panic(fmt.Errorf("no BAM index found for %s", b))
	}

	return bamIdxFh
}
