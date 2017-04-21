package size

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/biogo/hts/bam"
	"github.com/remeh/sizedwaitgroup"

	"github.com/omicsnut/binest"
)

// Run is the command line interface for binest size
func Run() {
	infile := flag.String("infile", "", "path to file with list of bam files")
	procs := flag.Int("procs", 1, "number of processors to use")
	flag.Parse()

	bampaths := make(chan string, 100)
	results := make(chan sizeInfo, 100)
	doneChan := make(chan bool, 1)

	go EstimateSize(bampaths, results, *procs)
	go writeResults(results, doneChan, os.Stdout)

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

			results <- sizeInfo{sampleName: si.Name, binData: normedData}

		}(bampath, sizes)
	}

	swg.Wait()
	close(sizes)
}

// writeResults writes to io.Writer after combining data from all samples
func writeResults(results <-chan sizeInfo, fin chan<- bool, outStream io.Writer) {
	mergedSizes := make(map[binest.RefBlock]map[string]float64)
	uniqSamples := make(map[string]bool)

	for result := range results {
		uniqSamples[result.sampleName] = true
		for _, refBlock := range result.binData.Blocks {
			if _, ok := mergedSizes[refBlock]; !ok {
				mergedSizes[refBlock] = make(map[string]float64)
			}
			mergedSizes[refBlock][result.sampleName] = result.binData.Bins[refBlock].Size
		}
	}

	samples := make([]string, 0, len(uniqSamples))
	for s := range uniqSamples {
		samples = append(samples, s)
	}

	sortedRefBlocks := getSortedBlocks(mergedSizes)
	fmt.Fprintf(outStream, "#CHROM\tSTART\tEND\t%s\n", strings.Join(samples, "\t"))

	for _, rBlock := range sortedRefBlocks {
		fmt.Fprintf(outStream, "%s\t%d\t%d\t%s\n",
			rBlock.Name, rBlock.Start, rBlock.End,
			getSizesString(mergedSizes[rBlock], samples))
	}

	fin <- true
}

// getSizesString gets the sizes of all samples as a string to be written
func getSizesString(m map[string]float64, samples []string) string {
	results := make([]string, len(samples))

	for idx, s := range samples {
		results[idx] = strconv.FormatFloat(m[s], 'f', -1, 64)
	}

	return strings.Join(results, "\t")
}

// getSortedBlocks sorts the ref blocks from the map
func getSortedBlocks(m map[binest.RefBlock]map[string]float64) []binest.RefBlock {
	blocks := make([]binest.RefBlock, 0, len(m))

	for rBlock := range m {
		blocks = append(blocks, rBlock)
	}

	sort.Slice(blocks, func(i, j int) bool {
		switch blocks[i].RefID - blocks[j].RefID {
		case 0:
			return blocks[i].Start < blocks[j].Start
		default:
			return blocks[i].RefID < blocks[j].RefID
		}
	})

	return blocks
}

// sizeInfo holds the normalized bin data and sample name
type sizeInfo struct {
	sampleName string
	binData    binest.NormBinData
}
