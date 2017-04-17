package binest

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"regexp"
	"sort"
	"strconv"

	"github.com/biogo/hts/bgzf"
	"github.com/biogo/store/interval"
	"github.com/brentp/xopen"
)

// BedRecord is a regex which will match chrom:start-end and chrom\tstart\tend
var BedRecord = regexp.MustCompile("(.+?)[:\t](\\d+)([\\-\t])(\\d+).*?")

// checkError checks for error and panics if present
func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

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

// ShuffleChunks shuffles BGZF chunks using the fisher yates method
func ShuffleChunks(c []bgzf.Chunk) {
	for i := range c {
		j := rand.Intn(i + 1)
		c[i], c[j] = c[j], c[i]
	}
}

// ReadBed takes a bed file and returns a map of int trees for overlap testing
func ReadBed(bedPath string, chromToID map[string]int) map[int]*interval.IntTree {
	rdr, err := xopen.Ropen(bedPath)
	checkError(err)

	tree := make(map[int]*interval.IntTree)
	bufRdr := bufio.NewReader(rdr)

	for {
		line, err := bufRdr.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		checkError(err)

		parsed := BedRecord.FindSubmatch(line)
		if len(parsed) != 5 {
			fmt.Errorf("Couldn't parse genomic region from bed line - %s", string(line))
		}
		chrom, start, isep, end := parsed[1], parsed[2], parsed[3], parsed[4]
		sChrom := string(chrom)
		intStart, err := strconv.Atoi(string(start))
		checkError(err)

		if bytes.Equal(isep, []byte{'-'}) {
			intStart--
		}

		if intStart < 0 {
			intStart = 0
		}
		intEnd, err := strconv.Atoi(string(end))
		checkError(err)

		if refID, ok := chromToID[sChrom]; ok {
			if _, ok := tree[refID]; !ok {
				tree[refID] = &interval.IntTree{}
			}
			tree[refID].Insert(&RefBlock{RefID: refID, Start: intStart, End: intEnd}, false)
		} else {
			fmt.Errorf("RefID for %s not found!", sChrom)
		}
	}

	return tree
}
