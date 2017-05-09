package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/omicsnut/binest"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {

	var (
		desc  = "Estimate copy number, size and sex from BAI/TBI index bins."
		app   = kingpin.New("binest", desc).Version(fmt.Sprintf("binest v%s", binest.Version))
		fai   = app.Flag("fai", "path to reference FAI index.").Short('f').ExistingFile()
		procs = app.Flag("cores", "number of cores to use.").Short('c').Default("1").Uint()

		copy = app.Command("copy", "Estimate per chromosome copy number from one or more indexes (stdin or arguments).")
		size = app.Command("size", "Compute size across 16kb bins from one or more indexes (stdin or arguments).")
		sex  = app.Command("sex", "Estimate sex genotype of a sample from one or more indexes (stdin or arguments).")

		cIdxs  = copy.Arg("index", "path to one or more indexes.").ExistingFiles()
		szIdxs = size.Arg("index", "path to one or more indexes.").ExistingFiles()
		sxIdxs = sex.Arg("index", "path to one or more indexes.").ExistingFiles()

		szRaw   = size.Flag("raw", "output raw sizes without normalization.").Short('r').Default("false").Bool()
		cPloidy = copy.Flag("ploidy", "ploidy to use for copy number estimate.").Short('p').Default("2").Uint()
		sPloidy = sex.Flag("ploidy", "ploidy to use for sex estimate.").Short('p').Default("2").Uint()
	)

	app.HelpFlag.Short('h')
	app.VersionFlag.Short('v')

	indexes := make(chan string, 100)
	doneChan := make(chan bool, 1)

	runtime.GOMAXPROCS(int(*procs))

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case "copy":
		go runCopy(indexes, doneChan, *fai, *cPloidy)

		if err := putIndexes(*cIdxs, indexes); err != nil {
			fmt.Fprintln(os.Stderr, "No indexes provided for copy number estimate!")
			app.Usage(os.Args[1:])
			os.Exit(1)
		}

		close(indexes)
		<-doneChan
	case "size":
		go runSize(indexes, doneChan, *fai, *szRaw)

		if err := putIndexes(*szIdxs, indexes); err != nil {
			fmt.Fprintln(os.Stderr, "No indexes provided for size calculation!")
			app.Usage(os.Args[1:])
			os.Exit(1)
		}

		close(indexes)
		<-doneChan
	case "sex":
		go runSex(indexes, doneChan, *fai, *sPloidy)

		if err := putIndexes(*sxIdxs, indexes); err != nil {
			fmt.Fprintln(os.Stderr, "No indexes provided for sex estimate!")
			app.Usage(os.Args[1:])
			os.Exit(1)
		}

		close(indexes)
		<-doneChan
	}

}
