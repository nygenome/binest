package cov

import (
	"flag"
	"fmt"
	"os"
)

// Run is the command line interface for binest cov
func Run() {
	bampath := flag.String("bam", "", "path to bam file")
	excludeBed := flag.String("excludeBed", "", "path to bed file with regions to exclude")
	flag.Parse()

	if *bampath == "" {
		fmt.Fprintln(os.Stderr, "No bam file provided!")
		flag.PrintDefaults()
		os.Exit(1)
	}

	EstimateCoverage(*bampath, *excludeBed)
}

// EstimateCoverage estimates the coverage excluding the regions in the bed file
func EstimateCoverage(bampath string, excludeBed string) {}
