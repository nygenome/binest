package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"

	"github.com/remeh/sizedwaitgroup"

	"github.com/omicsnut/binest"
)

// runCopy is the command line interface for binest copy
func runCopy(idxPaths <-chan string, finished chan<- bool, refs map[uint32]string, ploidy uint) {
	swg := sizedwaitgroup.New(runtime.GOMAXPROCS(0))

	sampleCopies := make(chan sampleCopy, 100)
	doneChan := make(chan bool, 1)

	go writeCopyEstimate(sampleCopies, doneChan)

	for idxPath := range idxPaths {
		swg.Add()

		go func(idx string, results chan<- sampleCopy) {
			defer swg.Done()

			bd, err := binest.NewBinData(idx)
			if err != nil {
				panic(err)
			}

			copies := bd.Copies(ploidy, refs)
			results <- sampleCopy{bd.Name, copies}

		}(idxPath, sampleCopies)

	}

	swg.Wait()
	close(sampleCopies)

	<-doneChan
	finished <- true
}

// writeCopyEstimate gets the copy estimate from sampleCopy stream and writes them to stdout
func writeCopyEstimate(results <-chan sampleCopy, finished chan<- bool) {
	stdout := bufio.NewWriter(os.Stdout)

	fmt.Fprintln(stdout, "SAMPLE\tCHROM\tCOPY_NUMBER\tNORMALIZED_SIZE")

	for result := range results {
		for _, copyEst := range result.copies {
			fmt.Fprintf(stdout, "%s\t%s\n", result.name, copyEst)
		}
	}

	stdout.Flush()
	finished <- true
}

// sampleCopy holds the name and []RefCopy for a sample
type sampleCopy struct {
	name   string
	copies []binest.RefCopy
}
