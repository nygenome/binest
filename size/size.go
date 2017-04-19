package size

import (
	"flag"
	"fmt"
	"os"

	"github.com/biogo/hts/bam"

	"github.com/omicsnut/binest"
)

// Run is the command line interface for binest cov
func Run() {
	infile := flag.String("infile", "", "path to file with list of bam files")
	flag.Parse()

	bampaths := make(chan string, 100)
	doneChan := make(chan bool, 1)

	go EstimateCoverage(bampaths, doneChan)

	for _, b := range flag.Args() {
		bampaths <- b
	}

	if binest.HasStdin() {
		binest.StreamLines(os.Stdin, bampaths)
	}

	if *infile != "" {
		fh, err := os.Open(*infile)
		binest.CheckError(err)
		binest.StreamLines(fh, bampaths)
	}

	close(bampaths)
	<-doneChan
}

// EstimateCoverage estimates the coverage excluding the regions in the bed file
func EstimateCoverage(bampaths <-chan string, doneChan chan<- bool) {
	results := make(chan binest.NormBinData, 100)

	go ProcessBamBins(bampaths, results)

	for result := range results {
		for _, refBlock := range result.Blocks {
			fmt.Fprintf(os.Stdout, "%s\t%d\t%d\t%.10f\n",
				refBlock.Name, refBlock.Start, refBlock.End,
				result.Bins[refBlock].Size)
		}
	}

	doneChan <- true
}

// ProcessBamBins reads bampaths from a channel and puts the per bin data to results channel
func ProcessBamBins(bampaths <-chan string, results chan<- binest.NormBinData) {
	for bampath := range bampaths {
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

		normedData, err := si.NormalizedBins()
		binest.CheckError(err)

		results <- normedData

		bamFh.Close()
		bamRdr.Close()
		bamIdxFh.Close()
	}

	close(results)
}
