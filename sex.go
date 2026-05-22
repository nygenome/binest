package binest

import (
	"fmt"
	"io"
)

// RunSex estimates the sex genotype along with norm X/Y sizes for all the
// given indexes read from a streaming IndexSource and writes results to io.Writer.
func RunSex(source IndexSource, w io.Writer, opts IndexOptions, ploidy uint, forceMF bool) error {
	if _, err := fmt.Fprintln(w, "SAMPLE\tESTIMATED_GENDER\tSEX_GENOTYPE\tNORM_X\tNORM_Y"); err != nil {
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

		sex, err := idx.Sex(ploidy, forceMF)
		if err != nil {
			batch.Add(idxPath, err)
			continue
		}

		if _, err = fmt.Fprintln(w, sex); err != nil {
			return err
		}
		if err := flushIfSupported(w); err != nil {
			return err
		}
	}
}
