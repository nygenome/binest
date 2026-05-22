package binest

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/biogo/hts/bam"
)

// RunNumreads counts the number of reads for each given index,
// read from a streaming IndexSource and writes results to io.Writer.
func RunNumreads(source IndexSource, w io.Writer, includeUnmapped bool) error {
	if _, err := fmt.Fprintln(w, "SAMPLE\tNUM_READS"); err != nil {
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

		sampleName := stripKnownSuffixes(idxPath)

		idx, err := readBamIndex(idxPath)
		if err != nil {
			batch.Add(idxPath, err)
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
			return err
		}
		if err := flushIfSupported(w); err != nil {
			return err
		}
	}
}

func readBamIndex(idxPath string) (*bam.Index, error) {
	fh, err := os.Open(idxPath)
	if err != nil {
		return nil, fmt.Errorf("open BAI %q: %w", idxPath, err)
	}

	idx, readErr := bam.ReadIndex(bufio.NewReader(fh))
	closeErr := fh.Close()
	if readErr != nil {
		return nil, errors.Join(
			fmt.Errorf("read BAI %q: %w", idxPath, readErr),
			closePathError("close BAI", idxPath, closeErr),
		)
	}
	if closeErr != nil {
		return nil, closePathError("close BAI", idxPath, closeErr)
	}
	return idx, nil
}
