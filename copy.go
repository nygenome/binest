package binest

import (
	"fmt"
	"io"
)

// RunChromCopy estimates the chromosome copy number for all the
// given indexes read from a channel and results written to io.Writer.
func RunChromCopy(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool, w io.Writer, faiPath string, ploidy uint) {
	fmt.Fprintln(w, "index_used\tchrom\tcopy_estimate\tnormalized_estimate")
	for idxPath := range idxsChan {
		idx, err := NewIndex(idxPath, faiPath)
		if err != nil {
			errChan <- err
			continue
		}

		fmt.Fprintln(w, idx.ChromCopy(ploidy))
	}
	doneChan <- true
}
