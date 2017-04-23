package sex

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/biogo/hts/bam"
	"github.com/biogo/hts/sam"
	"github.com/remeh/sizedwaitgroup"

	"github.com/omicsnut/binest"
)

// Run is the command line interface for binest sex
func Run() {
	infile := flag.String("infile", "", "path to file with list of bam files")
	ploidy := flag.Int("ploidy", 2, "Ploidy to use for estimation")
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
	results := make(chan sexEstimate, 100)
	doneChan := make(chan bool, 1)

	runtime.GOMAXPROCS(*procs)

	go EstimateSex(bampaths, results, *ploidy, *procs)
	go writeResults(results, doneChan, os.Stdout)

	var gotInput bool

	for _, b := range flag.Args() {
		bampaths <- b
		gotInput = true
	}

	if *infile != "" {
		fh, err := os.Open(*infile)
		binest.CheckError(err)
		binest.StreamLines(fh, bampaths)
		gotInput = true
	}

	if binest.HasStdin() {
		binest.StreamLines(os.Stdin, bampaths)
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

// EstimateSex estimates the sex of the samples from the BAM index
func EstimateSex(bampaths <-chan string, estimates chan<- sexEstimate, ploidy, procs int) {
	var swg sizedwaitgroup.SizedWaitGroup
	if procs == 1 {
		// To maintain input order use only one goroutine
		// equivalent to calling func without `go`
		swg = sizedwaitgroup.New(1)
	} else {
		swg = sizedwaitgroup.New(procs * 4)
	}

	for bampath := range bampaths {
		swg.Add()

		go func(b string, results chan<- sexEstimate) {
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

			estimate := getSexEstimate(normedData, si.RefMap, ploidy)
			estimate.sampleName = si.Name
			results <- estimate

		}(bampath, estimates)
	}

	swg.Wait()
	close(estimates)
}

// getSexEstimate gets the sexEstimate from the normalized bin data
func getSexEstimate(d binest.NormBinData, m map[int]*sam.Reference, ploidy int) sexEstimate {
	xSizes := make([]float64, 0, 16384)
	ySizes := make([]float64, 0, 16384)

	var chromPrefix string
	if strings.HasPrefix(m[d.Blocks[0].RefID].Name(), "chr") {
		chromPrefix = "chr"
	}

	for refBlock, binSize := range d.Bins {
		if m[refBlock.RefID].Name() == (chromPrefix+"X") && binSize > float64(0) {
			xSizes = append(xSizes, binSize)
		}
		if m[refBlock.RefID].Name() == (chromPrefix+"Y") && binSize > float64(0) {
			ySizes = append(ySizes, binSize)
		}
	}

	var (
		normXCopy float64
		normYCopy float64
		gender    string
		xCopy     uint32
		yCopy     uint32
	)

	if len(xSizes) > 0 {
		normXCopy = float64(ploidy) * binest.MedianFloat64(xSizes)
	}
	if len(ySizes) > 0 {
		normYCopy = float64(ploidy) * binest.MedianFloat64(ySizes)
	}

	if normXCopy >= float64(1.7) && normXCopy <= float64(2.3) && normYCopy <= float64(0.3) {
		gender = "female"
		xCopy = uint32(2)
		yCopy = uint32(0)
	} else if normXCopy >= float64(0.7) && normXCopy <= float64(1.3) && normYCopy >= float64(0.7) && normYCopy <= float64(1.3) {
		gender = "male"
		xCopy = uint32(1)
		yCopy = uint32(1)
	} else {
		gender = "unknown"
		xCopy = uint32(binest.Round(normXCopy, 0.7, 0))
		yCopy = uint32(binest.Round(normYCopy, 0.7, 0))
	}

	return sexEstimate{
		gender:    gender,
		xCopy:     xCopy,
		yCopy:     yCopy,
		normXCopy: normXCopy,
		normYCopy: normYCopy,
	}
}

func writeResults(results <-chan sexEstimate, fin chan<- bool, outStream io.Writer) {
	fmt.Println("SAMPLE\tESTIMATED_GENDER\tSEX_GENOTYPE\tESTIMATED_XSIZE\tESTIMATED_YSIZE")
	for result := range results {
		fmt.Fprintln(outStream, result)
	}
	fin <- true
}

// sexEstimate holds the result of the sex estimate of the sample
type sexEstimate struct {
	gender     string
	xCopy      uint32
	yCopy      uint32
	sampleName string
	normXCopy  float64
	normYCopy  float64
}

// String implements the Stringer interface for sexEstimate
func (s sexEstimate) String() string {
	var sexGenotype string
	for i := 0; i < int(s.xCopy); i++ {
		sexGenotype += "X"
	}
	for i := 0; i < int(s.yCopy); i++ {
		sexGenotype += "Y"
	}
	if s.gender == "unknown" && len(sexGenotype) == 1 {
		sexGenotype += "O"
	}

	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s",
		s.sampleName, s.gender, sexGenotype,
		strconv.FormatFloat(s.normXCopy, 'f', -1, 64),
		strconv.FormatFloat(s.normYCopy, 'f', -1, 64))
}
