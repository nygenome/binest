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
	defer func() {
		doneChan <- true
	}()

	if _, err := fmt.Fprintln(w, "SAMPLE\tNUM_READS"); err != nil {
		errChan <- err
		return
	}

	for idxPath := range idxsChan {
		sampleName := stripKnownSuffixes(idxPath)

		idx, err := readBamIndex(idxPath)
		if err != nil {
			errChan <- err
			continue
		}

		sampleRdCnt := uint64(0)
		for n := 0; n < idx.NumRefs(); n++ {
			chromStats, ok := idx.ReferenceStats(n)
			if !ok {
				continue
			}

			sampleRdCnt += chromStats.Mapped
			if includeUnmapped {
				sampleRdCnt += chromStats.Unmapped
			}
		}

		if _, err = fmt.Fprintf(w, "%s\t%d\n", sampleName, sampleRdCnt); err != nil {
			errChan <- err
			return
		}
	}
}

func readBamIndex(idxPath string) (*bam.Index, error) {
	fh, err := os.Open(idxPath)
	if err != nil {
		return nil, err
	}

	idx, readErr := bam.ReadIndex(bufio.NewReader(fh))
	closeErr := fh.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return idx, nil
}
