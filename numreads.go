package binest

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/biogo/hts/bam"
)

// RunNumreads counts the number of reads for each given index,
// read from the channel and results written to io.Writer.
func RunNumreads(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool, w io.Writer, includeUnmapped bool) {
	fmt.Fprintln(w, "SAMPLE\tNUM_READS")
	for idxPath := range idxsChan {
		sampleName := stripKnownSuffixes(idxPath)

		fh, err := os.Open(idxPath)
		if err != nil {
			errChan <- err
			continue
		}
		defer fh.Close()

		idx, err := bam.ReadIndex(bufio.NewReader(fh))
		if err != nil {
			errChan <- err
			continue
		}

		sampleRdCnt := uint64(0)
		for n := 0; n <= idx.NumRefs(); n++ {
			chromStats, ok := idx.ReferenceStats(n)
			if !ok {
				continue
			}

			sampleRdCnt += chromStats.Mapped
			if includeUnmapped {
				sampleRdCnt += chromStats.Unmapped
			}
		}

		fmt.Fprintf(w, "%s\t%d\n", sampleName, sampleRdCnt)
	}

	doneChan <- true
}
