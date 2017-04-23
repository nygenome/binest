package copy

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
	"github.com/omicsnut/binest"
	"github.com/remeh/sizedwaitgroup"
)

// Run is the command line interface to binest copy
func Run() {
	infile := flag.String("infile", "", "path to file with list of bam files")
	ploidy := flag.Int("ploidy", 2, "Ploidy to use for copy number estimation")
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
	results := make(chan copyEstimate, 100)
	doneChan := make(chan bool, 1)

	runtime.GOMAXPROCS(*procs)

	go EstimateCopy(bampaths, results, *ploidy, *procs)
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

// EstimateCopy estimates the per chrom copy number
func EstimateCopy(bampaths <-chan string, estimates chan<- copyEstimate, ploidy, procs int) {
	swg := sizedwaitgroup.New(procs * 4)

	for bampath := range bampaths {

		swg.Add()

		go func(b string, results chan<- copyEstimate) {
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

			estimate := getCopyEstimate(normedData, si.RefMap, ploidy)
			estimate.sampleName = si.Name
			results <- estimate

		}(bampath, estimates)

	}

	swg.Wait()
	close(estimates)
}

// getCopyEstimate gets the per sample copy estimate from normalized bin data
func getCopyEstimate(d binest.NormBinData, m map[int]*sam.Reference, ploidy int) copyEstimate {
	chroms := make([]string, 0, len(m))
	dummy := make([]bool, len(m))

	var (
		chrom         string
		normChromCopy float64
		estChromCopy  uint32
	)

	for idx := range dummy {
		chrom = m[idx].Name()
		if strings.HasPrefix(chrom, "GL") ||
			strings.HasPrefix(chrom, "HLA") ||
			strings.HasSuffix(chrom, "random") ||
			strings.HasSuffix(chrom, "decoy") ||
			strings.HasSuffix(chrom, "EBV") ||
			strings.HasSuffix(chrom, "alt") {
			continue
		}
		chroms = append(chroms, chrom)
	}

	sizes := make(map[string][]float64, len(chroms))
	estimates := make(map[string]chromEstimate, len(chroms))

	for refBlock, binSize := range d.Bins {
		if binSize <= float64(0) {
			continue
		}

		chrom = m[refBlock.RefID].Name()
		if strings.HasPrefix(chrom, "GL") ||
			strings.HasPrefix(chrom, "HLA") ||
			strings.HasSuffix(chrom, "random") ||
			strings.HasSuffix(chrom, "decoy") ||
			strings.HasSuffix(chrom, "EBV") ||
			strings.HasSuffix(chrom, "alt") {
			continue
		}

		if _, ok := sizes[chrom]; ok {
			sizes[chrom] = append(sizes[chrom], binSize)
		} else {
			sizes[chrom] = make([]float64, 1, 16384)
			sizes[chrom][0] = binSize
		}
	}

	for chrom, chromSizes := range sizes {
		if len(chromSizes) > 2 {
			normChromCopy = float64(ploidy) * binest.MedianFloat64(chromSizes)
			estChromCopy = uint32(binest.Round(normChromCopy, 0.7, 0))
			estimates[chrom] = chromEstimate{normCopy: normChromCopy, estCopy: estChromCopy}
		} else if len(chromSizes) == 1 {
			normChromCopy = float64(ploidy) * chromSizes[0]
			estChromCopy = uint32(binest.Round(normChromCopy, 0.7, 0))
			estimates[chrom] = chromEstimate{normCopy: normChromCopy, estCopy: estChromCopy}
		} else {
			estimates[chrom] = chromEstimate{}
		}
	}

	return copyEstimate{chroms: chroms, estimates: estimates}
}

// writeResults writes copy estimate results to io writer
func writeResults(results <-chan copyEstimate, fin chan<- bool, outStream io.Writer) {
	fmt.Println("SAMPLE\tCHROM\tCOPY_NUMBER\tESTIMATED_SIZE")
	for result := range results {
		fmt.Fprintln(outStream, result)
	}
	fin <- true
}

// copyEstimate holds the copy number estimate for a sample
type copyEstimate struct {
	chroms     []string
	sampleName string
	estimates  map[string]chromEstimate
}

// chromEstimate holds the copy number estimate of one chromosome
type chromEstimate struct {
	normCopy float64
	estCopy  uint32
}

// String implements the Stringer interface for copyEstimate
func (c copyEstimate) String() string {
	results := make([]string, len(c.chroms))

	for idx, chrom := range c.chroms {
		results[idx] = fmt.Sprintf("%s\t%s\t%d\t%s",
			c.sampleName, chrom, c.estimates[chrom].estCopy,
			strconv.FormatFloat(c.estimates[chrom].normCopy, 'f', -1, 64))
	}

	return strings.Join(results, "\n")
}
