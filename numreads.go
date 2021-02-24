package binest

import (
	"fmt"
	"io"
)

// RunNumreads counts the number of reads for each given index,
// read from the channel and results written to io.Writer.
func RunNumreads(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool, w io.Writer, includeUnmapped bool) {
	fmt.Fprintln(w, "SAMPLE\tNUM_READS")
	for idxPath := range idxsChan {
		internalIdx, err := baiRefIdxs(idxPath)
		if err != nil {
			errChan <- err
			continue
		}

		sampleRdCnt := uint64(0)
		for _, chrom := range internalIdx {
			sampleRdCnt += chrom.Stats.Mapped
			if includeUnmapped {
				sampleRdCnt += chrom.Stats.Unmapped
			}
		}

		sampleName := stripKnownSuffixes(idxPath)
		fmt.Fprintf(w, "%s\t%d\n", sampleName, sampleRdCnt)
	}

	doneChan <- true
}
