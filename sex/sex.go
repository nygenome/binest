package sex

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/biogo/hts/bam"

	"github.com/omicsnut/binest"
)

// Run is the command line interface for binest sex
func Run() {
	infile := flag.String("in", "", "path to file with list of bam files")
	sexChroms := flag.String("chroms", "X,Y", "comma separated X and Y chrom names in reference")
	flag.Parse()

	bampaths := make(chan string, 100)
	results := make(chan sexEstimate, 100)
	doneChan := make(chan bool, 1)

	go EstimateSex(bampaths, results, strings.Split(*sexChroms, ","))
	go writeResults(results, doneChan, os.Stdout)

	for _, b := range flag.Args() {
		bampaths <- b
	}

	if *infile != "" {
		fh, err := os.Open(*infile)
		binest.CheckError(err)
		binest.StreamLines(fh, bampaths)
	}

	if binest.HasStdin() {
		binest.StreamLines(os.Stdin, bampaths)
	}

	close(bampaths)
	<-doneChan
}

// EstimateSex estimates the sex of the samples from the BAM index
func EstimateSex(bampaths <-chan string, results chan<- sexEstimate, sChroms []string) {
	for bampath := range bampaths {
		func() {
			bamFh, err := os.Open(bampath)
			binest.CheckError(err)
			defer bamFh.Close()

			bamIdxFh, err := os.Open(fmt.Sprintf("%s.bai", bampath))
			if err != nil {
				bamIdxFh, err = os.Open(bampath[:len(bampath)-4] + ".bai")
				binest.CheckError(err)
			}
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

			estimate := getSexEstimate(normedData, sChroms)
			estimate.sampleName = si.Name
			results <- estimate
		}()
	}

	close(results)
}

// getSexEstimate gets the sexEstimate from the normalized bin data
func getSexEstimate(d binest.NormBinData, sChroms []string) sexEstimate {
	xSizes := make([]float64, 0, 16384)
	ySizes := make([]float64, 0, 16384)

	for refBlock, nBin := range d.Bins {
		if refBlock.Name == sChroms[0] && nBin.Size > float64(0) {
			xSizes = append(xSizes, nBin.Size)
		}
		if refBlock.Name == sChroms[1] && nBin.Size > float64(0) {
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
		xCopy = uint8(xMedian)
		yCopy = uint8(yMedian)
	}

	return sexEstimate{
		gender:  gender,
		xCopy:   xCopy,
		yCopy:   yCopy,
		xMedian: xMedian,
		yMedian: yMedian,
	}
}

func writeResults(results <-chan sexEstimate, fin chan<- bool, outStream *os.File) {
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
