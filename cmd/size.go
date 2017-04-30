package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"

	"github.com/remeh/sizedwaitgroup"

	"github.com/omicsnut/binest"
)

// runSize is the command line interface for binest size
func runSize(idxPaths <-chan string, finished chan<- bool, faiPath string, raw bool) {
	swg := sizedwaitgroup.New(runtime.GOMAXPROCS(0))

	sampleSizes := make(chan sampleSize, 100)
	doneChan := make(chan bool, 1)

	go writeSizeResult(sampleSizes, doneChan)

	if raw {
		fmt.Fprintln(os.Stdout, "SAMPLE\tCHROM\tSTART\tEND\tRAW_SIZE")
	} else {
		fmt.Fprintln(os.Stdout, "SAMPLE\tCHROM\tSTART\tEND\tNORMALIZED_SIZE")
	}

	for idxPath := range idxPaths {
		swg.Add()

		go func(idx, fai string, results chan<- sampleSize) {
			defer swg.Done()

			bd, err := binest.NewBinData(idx, fai)
			if err != nil {
				panic(err)
			}

			if raw {
				results <- sampleSize{bd.Name, bd.Raw()}
			} else {
				results <- sampleSize{bd.Name, bd.Normalized()}
			}

		}(idxPath, faiPath, sampleSizes)

	}

	swg.Wait()
	close(sampleSizes)

	<-doneChan
	finished <- true
}

// writeSizeResult gets the size result from sampleSize stream and writes them to stdout
func writeSizeResult(results <-chan sampleSize, finished chan<- bool) {
	stdout := bufio.NewWriter(os.Stdout)

	for result := range results {
		for _, bin := range result.sizes {
			fmt.Fprintf(stdout, "%s\t%s\n", result.name, bin)
		}
	}

	stdout.Flush()
	finished <- true
}

// sampleSize holds the name and []Bin for a sample
type sampleSize struct {
	name  string
	sizes []binest.Bin
}
