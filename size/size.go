package size

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/biogo/hts/bam"
	"github.com/remeh/sizedwaitgroup"

	"github.com/omicsnut/binest"
)

// Run is the command line interface for binest size
func Run() {
	infile := flag.String("infile", "", "path to file with list of bam files")
	procs := flag.Int("procs", 1, "number of processors to use")
	flag.Parse()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Usage 1: %s -procs 4 BAM1 BAM2 ...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Usage 2: %s -infile FILE_WITH_LIST_OF_BAM_FILES\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Usage 3: `pipe bamfile paths` | %s \n\n", os.Args[0])
		fmt.Fprint(os.Stderr, "Output is written to STDOUT\n\nOptions:\n")
		flag.PrintDefaults()
	}

	bampaths := make(chan string, 100)
	results := make(chan sizeInfo, 100)
	doneChan := make(chan bool, 1)

	go EstimateSize(bampaths, results, *procs)
	go writeResults(results, doneChan, os.Stdout)

	var gotInput bool

	for _, b := range flag.Args() {
		bampaths <- b
		gotInput = true
	}

	if binest.HasStdin() {
		binest.StreamLines(os.Stdin, bampaths)
		gotInput = true
	}

	if *infile != "" {
		fh, err := os.Open(*infile)
		binest.CheckError(err)
		binest.StreamLines(fh, bampaths)
		gotInput = true
	}

	close(bampaths)

	if !gotInput {
		fmt.Fprintln(os.Stderr, "No bam files provided to process!")
		flag.Usage()
		os.Exit(1)
	}

	<-doneChan
}

// EstimateSize gets the normalized bin sizes of samples possibly concurrently
func EstimateSize(bampaths <-chan string, sizes chan<- sizeInfo, procs int) {
	swg := sizedwaitgroup.New(procs * 4)

	for bampath := range bampaths {
		swg.Add()

		go func(b string, results chan<- sizeInfo) {
			defer swg.Done()

			bamFh, err := os.Open(b)
			binest.CheckError(err)
			defer bamFh.Close()

			bamIdxFh := binest.ReadIndex(b)
			defer bamIdxFh.Close()

			bamRdr, err := bam.NewReader(bamFh, 2)
			binest.CheckError(err)
			defer bamRdr.Close()

			bamIdx, err := bam.ReadIndex(bamIdxFh)
			binest.CheckError(err)

			si, err := binest.NewSampleIndex(bamIdx, bamRdr.Header())
			binest.CheckError(err)

			normedData, err := si.NormalizedBins()
			binest.CheckError(err)

			for _, rBlock := range normedData.Blocks {
				results <- sizeInfo{
					sample: si.Name,
					start:  rBlock.Start,
					end:    rBlock.End,
					rName:  si.RefMap[rBlock.RefID].Name(),
					size:   normedData.Bins[rBlock] - 1,
				}
			}

		}(bampath, sizes)
	}

	swg.Wait()
	close(sizes)
}

// writeResults writes to io.Writer after combining data from all samples
func writeResults(results <-chan sizeInfo, fin chan<- bool, outStream io.Writer) {
	fmt.Println("SAMPLE\tCHROM\tSTART\tEND\tLOG2_NORMALIZED_SIZE")
	for result := range results {
		fmt.Fprintln(outStream, result)
	}

	fin <- true
}

// sizeInfo holds the normalized bin data and sample name
type sizeInfo struct {
	sample string
	rName  string
	start  int
	end    int
	size   float64
}

// String implements the Stringer interface for sizeInfo
func (s sizeInfo) String() string {
	return fmt.Sprintf("%s\t%s\t%d\t%d\t%s",
		s.sample, s.rName, s.start, s.end,
		strconv.FormatFloat(s.size, 'f', -1, 64))
}
