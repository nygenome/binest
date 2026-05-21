package binest

import (
	"fmt"
	"io"
)

// RunSize estimates the per window sizes for all the given indexes
// read from a channel and results written to io.Writer.
func RunSize(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool, w io.Writer, faiPath string, rawSize bool) {
	defer func() {
		doneChan <- true
	}()

	sizeString := "NORMALIZED_SIZE"
	if rawSize {
		sizeString = "RAW_SIZE"
	}
	if _, err := fmt.Fprintf(w, "CHROM\tSTART\tEND\t%s\tSAMPLE\n", sizeString); err != nil {
		errChan <- err
		return
	}

	for idxPath := range idxsChan {
		idx, err := NewIndex(idxPath, faiPath)
		if err != nil {
			errChan <- err
			continue
		}

		sizes, err := idx.Sizes(rawSize)
		if err != nil {
			errChan <- err
			continue
		}

		if _, err = fmt.Fprintln(w, sizes); err != nil {
			errChan <- err
			return
		}
	}
}
