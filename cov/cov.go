package cov

import (
	"flag"
	"fmt"
	"os"

	"github.com/biogo/hts/bam"

	"github.com/omicsnut/binest"
)

// Run is the command line interface for binest cov
func Run() {
	bampath := flag.String("bam", "", "path to bam file")
	flag.Parse()

	if *bampath == "" {
		fmt.Fprintln(os.Stderr, "No bam file provided!")
		flag.PrintDefaults()
		os.Exit(1)
	}

	EstimateCoverage(*bampath)
}

// EstimateCoverage estimates the coverage excluding the regions in the bed file
func EstimateCoverage(bampath string) {
	bamFh, err := os.Open(bampath)
	binest.CheckError(err)

	bamIdxFh, err := os.Open(fmt.Sprintf("%s.bai", bampath))
	binest.CheckError(err)

	bamRdr, err := bam.NewReader(bamFh, 2)
	binest.CheckError(err)

	bamIdx, err := bam.ReadIndex(bamIdxFh)
	binest.CheckError(err)

	si, err := binest.NewSampleIndex(bamIdx, bamRdr.Header())
	binest.CheckError(err)

	bins, blocks, err := si.NormalizedBins()
	binest.CheckError(err)

	for _, refBlock := range blocks {
		binInfo := bins[refBlock]
		fmt.Fprintf(os.Stdout, "%s\t%d\t%d\t%.10f\n",
			refBlock.Name, refBlock.Start, refBlock.End, binInfo.Size)
	}
}
