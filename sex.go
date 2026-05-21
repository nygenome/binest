package binest

import (
	"fmt"
	"io"
)

// RunSex estimates the sex genotype along with norm X/Y sizes for all the
// given indexes read from the channel and results written to io.Writer.
func RunSex(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool, w io.Writer, faiPath string, ploidy uint, forceMF bool) {
	defer func() {
		doneChan <- true
	}()

	if _, err := fmt.Fprintln(w, "SAMPLE\tESTIMATED_GENDER\tSEX_GENOTYPE\tNORM_X\tNORM_Y"); err != nil {
		errChan <- err
		return
	}

	for idxPath := range idxsChan {
		idx, err := NewIndex(idxPath, faiPath)
		if err != nil {
			errChan <- err
			continue
		}

		sex, err := idx.Sex(ploidy, forceMF)
		if err != nil {
			errChan <- err
			continue
		}

		if _, err = fmt.Fprintln(w, sex); err != nil {
			errChan <- err
			return
		}
	}
}
