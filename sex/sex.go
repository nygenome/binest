package sex

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/biogo/hts/bam"
	"github.com/remeh/sizedwaitgroup"

	"github.com/omicsnut/binest"
)

// BinestSexVersion is the tagged version of sex subcommand for feature and bug tracking
const BinestSexVersion = "0.1"

// Run is the command line interface for binest sex
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
	results := make(chan sexEstimate, 100)
	doneChan := make(chan bool, 1)

	runtime.GOMAXPROCS(*procs)

	go EstimateSex(bampaths, results, *procs)
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
		flag.Usage()
		os.Exit(1)
	}

	<-doneChan
}

// EstimateSex estimates the sex of the samples from the BAM index
func EstimateSex(bampaths <-chan string, estimates chan<- sexEstimate, procs int) {
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

			estimate := getSexEstimate(normedData)
			estimate.sampleName = si.Name
			results <- estimate

		}(bampath, estimates)
	}

	swg.Wait()
	close(estimates)
}

// getSexEstimate gets the sexEstimate from the normalized bin data
func getSexEstimate(d binest.NormBinData) sexEstimate {
	xSizes := make([]float64, 0, 16384)
	ySizes := make([]float64, 0, 16384)

	var chromPrefix string
	if strings.HasPrefix(d.Blocks[0].Name, "chr") {
		chromPrefix = "chr"
	}

	for refBlock, nBin := range d.Bins {
		if refBlock.Name == (chromPrefix+"X") && nBin.Size > float64(0) {
			xSizes = append(xSizes, nBin.Size)
		}
		if refBlock.Name == (chromPrefix+"Y") && nBin.Size > float64(0) {
			ySizes = append(ySizes, nBin.Size)
		}
	}

	var (
		xMedian float64
		yMedian float64
		gender  string
		xCopy   uint8
		yCopy   uint8
	)

	sort.Slice(xSizes, func(i, j int) bool { return xSizes[i] < xSizes[j] })
	sort.Slice(ySizes, func(i, j int) bool { return ySizes[i] < ySizes[j] })

	// Assuming ploidy of 2
	if len(xSizes) > 0 {
		xMedian = float64(2) * xSizes[int(float64(len(xSizes))/2)]
	}
	if len(ySizes) > 0 {
		yMedian = float64(2) * ySizes[int(float64(len(ySizes))/2)]
	}

	if xMedian >= float64(1.7) && xMedian <= float64(2.3) && yMedian <= float64(0.3) {
		gender = "female"
		xCopy = uint8(2)
		yCopy = uint8(0)
	} else if xMedian >= float64(0.7) && xMedian <= float64(1.3) && yMedian >= float64(0.7) && yMedian <= float64(1.3) {
		gender = "male"
		xCopy = uint8(1)
		yCopy = uint8(1)
	} else {
		gender = "unknown"
		xCopy = uint8(binest.Round(xMedian, 0.7, 0))
		yCopy = uint8(binest.Round(yMedian, 0.7, 0))
	}

	return sexEstimate{
		gender:  gender,
		xCopy:   xCopy,
		yCopy:   yCopy,
		xMedian: xMedian,
		yMedian: yMedian,
	}
}

func writeResults(results <-chan sexEstimate, fin chan<- bool, outStream io.Writer) {
	fmt.Fprintf(os.Stderr, "#binest sex version %s\n", BinestSexVersion)
	fmt.Println("#SAMPLE\tESTIMATED_GENDER\tSEX_GENOTYPE\tNORMALIZED_XMEAN\tNORMALIZED_YMEAN")
	for result := range results {
		fmt.Fprintln(outStream, result)
	}
	fin <- true
}

// sexEstimate holds the result of the sex estimate of the sample
type sexEstimate struct {
	sampleName string
	gender     string
	xCopy      uint8
	yCopy      uint8
	xMedian    float64
	yMedian    float64
}

// String implements the Stringer interface for sexEstimate
func (s sexEstimate) String() string {
	var out string
	for i := 0; i < int(s.xCopy); i++ {
		out += "X"
	}
	for i := 0; i < int(s.yCopy); i++ {
		out += "Y"
	}
	if s.gender == "unknown" && len(out) == 1 {
		out += "O"
	}

	return fmt.Sprintf("%s\t%s\t%s\t%.10f\t%.10f",
		s.sampleName, s.gender, out, s.xMedian, s.yMedian)
}
