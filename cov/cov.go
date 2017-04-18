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
	checkError(err)

	bamIdxFh, err := os.Open(fmt.Sprintf("%s.bai", bampath))
	checkError(err)

	bamRdr, err := bam.NewReader(bamFh, 2)
	checkError(err)

	bamIdx, err := bam.ReadIndex(bamIdxFh)
	checkError(err)

	si, err := binest.NewSampleIndex(bamIdx, bamRdr.Header())
	checkError(err)

	bins, err := si.NormalizedBins()
	checkError(err)

	for i := 0; i < len(bins); i++ {
		fmt.Fprintln(os.Stdout, bins[i])
	}
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}
