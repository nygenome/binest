package binest

import (
	"fmt"
	"io"
)

// RunSize estimates the per window sizes for all the given indexes
// read from a streaming IndexSource and writes results to io.Writer.
func RunSize(source IndexSource, w io.Writer, opts IndexOptions, rawSize bool) error {
	sizeString := "NORMALIZED_SIZE"
	if rawSize {
		sizeString = "RAW_SIZE"
	}
	if _, err := fmt.Fprintf(w, "CHROM\tSTART\tEND\t%s\tSAMPLE\n", sizeString); err != nil {
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

		sizes, err := idx.Sizes(rawSize)
		if err != nil {
			batch.Add(idxPath, err)
			continue
		}

		if _, err = fmt.Fprintln(w, sizes); err != nil {
			return err
		}
		if err := flushIfSupported(w); err != nil {
			return err
		}
	}
}
