package binest

import (
	"fmt"
	"io"
)

// RunChromCopy estimates the chromosome copy number for all the
// given indexes read from a streaming IndexSource and writes results to io.Writer.
func RunChromCopy(source IndexSource, w io.Writer, opts IndexOptions, ploidy uint) error {
	if _, err := fmt.Fprintln(w, "SAMPLE\tCHROM\tCOPY_ESTIMATE\tNORM_ESTIMATE"); err != nil {
		return err
	}
	if err := flushIfSupported(w); err != nil {
		return err
	}

	var batch BatchError
	for {
		idxPath, ok, err := source.Next()
		if err != nil {
			batch.Add("", err)
			return batch.Err()
		}
		if !ok {
			return batch.Err()
		}

		idx, err := NewIndexWithOptions(idxPath, opts)
		if err != nil {
			batch.Add(idxPath, err)
			continue
		}

		copies, err := idx.ChromCopy(ploidy)
		if err != nil {
			batch.Add(idxPath, err)
			continue
		}

		if _, err = fmt.Fprintln(w, copies); err != nil {
			return err
		}
		if err := flushIfSupported(w); err != nil {
			return err
		}
	}
}
