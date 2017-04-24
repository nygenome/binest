package chunk

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/biogo/hts/bam"
	"github.com/omicsnut/binest"
	"github.com/remeh/sizedwaitgroup"
)

// Run is the command line interface to binest chunk
func Run() {
	infile := flag.String("infile", "", "path to file with list of bam files")
	numChunks := flag.Int("nchunks", 100, "number of chunks to split data into")
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
	results := make(chan chunkInfo, 100)
	doneChan := make(chan bool, 1)

	go EstimateChunks(bampaths, results, *numChunks, *procs)
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

// EstimateChunks writes the chunk info such that all BAM files have ~even data in them
func EstimateChunks(bampaths <-chan string, chunks chan<- chunkInfo, numChunks, procs int) {
	swg := sizedwaitgroup.New(procs * 4)

	rawBinData := make(chan binest.RawBinData, 100)
	go mergeBins(rawBinData, chunks, numChunks)

	for bampath := range bampaths {
		swg.Add()

		go func(b string, results chan<- chunkInfo) {
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

			rawData, err := si.RawBins()
			binest.CheckError(err)

			rawBinData <- rawData

		}(bampath, chunks)
	}

	swg.Wait()
	close(rawBinData)
}

// mergeBins sums the per bin size across all samples
func mergeBins(bins <-chan binest.RawBinData, chunks chan<- chunkInfo, numChunks int) {
	var totalSize int64
	binSizes := make(map[binest.RefBlock]int64)

	var size int64
	for binData := range bins {
		for blockIdx, refBlock := range binData.Blocks {
			size = binData.Sizes[blockIdx]
			totalSize += size
			if _, ok := binSizes[refBlock]; ok {
				binSizes[refBlock] += size
			} else {
				binSizes[refBlock] = size
			}
		}
	}

	// TODO add chunk impl

	close(chunks)
}

// sortBlockKeys sorts the refblocks in the map keys
func sortBlockKeys(m map[binest.RefBlock]int64) []binest.RefBlock {
	blocks := make([]binest.RefBlock, 0, len(m))

	for k := range m {
		blocks = append(blocks, k)
	}

	sort.Slice(blocks, func(i, j int) bool {
		switch blocks[i].RefID - blocks[j].RefID {
		case 0:
			return blocks[i].Start < blocks[j].End
		default:
			return blocks[i].RefID < blocks[j].RefID
		}
	})

	return blocks
}

// writeResults writes the chunk results to io writer
func writeResults(results <-chan chunkInfo, fin chan<- bool, outStream io.Writer) {
	fmt.Println("CHROM\tSTART\tEND\tAPPROXIMATE_SIZE")
	for result := range results {
		fmt.Fprintln(outStream, result)
	}

	fin <- true
}

// chunkInfo holds the interval data per chunk
type chunkInfo struct {
	chrom string
	start int
	end   int
	size  int
}

// String implements the Stringer interface for chunkInfo
func (c chunkInfo) String() string {
	return fmt.Sprintf("%s\t%d\t%d\t%d", c.chrom, c.start, c.end, c.size)
}
