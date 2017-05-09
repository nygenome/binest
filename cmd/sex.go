package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"

	"github.com/remeh/sizedwaitgroup"

	"github.com/omicsnut/binest"
)

// runSex is the command line interface for binest size
func runSex(idxPaths <-chan string, finished chan<- bool, faiPath string, ploidy uint) {
	swg := sizedwaitgroup.New(runtime.GOMAXPROCS(0))

	sampleSexes := make(chan binest.SexEstimate, 100)
	doneChan := make(chan bool, 1)

	go writeSexEstimate(sampleSexes, doneChan)

	for idxPath := range idxPaths {
		swg.Add()

		go func(idx string, results chan<- binest.SexEstimate) {
			defer swg.Done()

			bd, err := binest.NewBinData(idx)
			if err != nil {
				panic(err)
			}

			refs, err := binest.GetRefMap(faiPath, idx)
			if err != nil {
				panic(err)
			}

			results <- bd.DetectSex(ploidy, refs)

		}(idxPath, sampleSexes)

	}

	swg.Wait()
	close(sampleSexes)

	<-doneChan
	finished <- true
}

// writeSexEstimate gets the sex estimate from sampleCopy stream and writes them to stdout
func writeSexEstimate(results <-chan binest.SexEstimate, finished chan<- bool) {
	stdout := bufio.NewWriter(os.Stdout)

	fmt.Fprintln(stdout, "SAMPLE\tESTIMATED_GENDER\tSEX_GENOTYPE\tNORMALIZED_XSIZE\tNORMALIZED_YSIZE")

	for result := range results {
		fmt.Fprintln(stdout, result)
	}

	stdout.Flush()
	finished <- true
}
