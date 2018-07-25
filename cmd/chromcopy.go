package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"

	"github.com/remeh/sizedwaitgroup"

	"git.nygenome.org/rmusunuri/binest"
)

// runChromCopy is the command line interface for binest ploidy
func runChromCopy(idxPaths <-chan string, finished chan<- bool, faiPath string, ploidy uint) {
	swg := sizedwaitgroup.New(runtime.GOMAXPROCS(0))

	sampleChromCopies := make(chan sampleChromCopy, 100)
	doneChan := make(chan bool, 1)

	go writeChromCopyEstimate(sampleChromCopies, doneChan)

	for idxPath := range idxPaths {
		swg.Add()

		go func(idx string, results chan<- sampleChromCopy) {
			defer swg.Done()

			bd, err := binest.NewBinData(idx)
			if err != nil {
				panic(err)
			}

			refs, err := binest.GetRefMap(faiPath, idx)
			if err != nil {
				panic(err)
			}

			copies := bd.Copies(ploidy, refs)
			results <- sampleChromCopy{bd.Name, copies}

		}(idxPath, sampleChromCopies)

	}

	swg.Wait()
	close(sampleChromCopies)

	<-doneChan
	finished <- true
}

// writeChromCopyEstimate gets the ploidy estimate from sampleChromCopy stream and writes them to stdout
func writeChromCopyEstimate(results <-chan sampleChromCopy, finished chan<- bool) {
	stdout := bufio.NewWriter(os.Stdout)

	fmt.Fprintln(stdout, "SAMPLE\tCHROM\tCOPY_NUMBER\tNORMALIZED_SIZE")

	for result := range results {
		for _, est := range result.copies {
			fmt.Fprintf(stdout, "%s\t%s\n", result.name, est)
		}
	}

	stdout.Flush()
	finished <- true
}

// sampleChromCopy holds the name and []RefCopy for a sample
type sampleChromCopy struct {
	name   string
	copies []binest.RefCopy
}
