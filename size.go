package binest

import (
	"fmt"
	"io"
)

// RunSize estimates the per window sizes for all the given indexes
// read from a channel and results written to io.Writer.
func RunSize(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool, w io.Writer, faiPath string, rawSize bool) {
	sizeString := "NORMALIZED_SIZE"
	if rawSize {
		sizeString = "RAW_SIZE"
	}
	fmt.Fprintf(w, "CHROM\tSTART\tEND\t%s\tINDEX_USED\n", sizeString)

	for idxPath := range idxsChan {
		idx, err := NewIndex(idxPath, faiPath)
		if err != nil {
			errChan <- err
			continue
		}

		fmt.Fprintln(w, idx.Sizes(rawSize))
	}
	doneChan <- true
}
