package main

import (
	"fmt"
	"os"

	"github.com/omicsnut/binest"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {

	var (
		desc    = "Estimate copy number, density and sex from BAI/TBI index bins."
		app     = kingpin.New("binest", desc).Version(fmt.Sprintf("binest v%s", binest.Version))
		faiPath = app.Flag("fai", "path to reference FAI index.").ExistingFile()
		nProcs  = app.Flag("procs", "number of cores to use.").Default("1").Uint()

		copyCmd    = app.Command("copy", "Estimate per chromosome copy number with BAI index.")
		copyIdxs   = copyCmd.Arg("index", "path to one or more BAI indexes.").ExistingFiles()
		copyPloidy = copyCmd.Flag("ploidy", "ploidy to use for copy number estimate.").Default("2").Uint()

		densCmd  = app.Command("density", "Compute density across 16kb bins with BAI/TBI index.")
		densIdxs = densCmd.Arg("index", "path to one or more BAI/TBI indexes.").ExistingFiles()
		densRaw  = densCmd.Flag("raw", "output raw sizes without normalization.").Default("false").Bool()

		sexCmd    = app.Command("sex", "Estimate sex genotype of a sample with BAI index.")
		sexIdxs   = sexCmd.Arg("index", "path to one or more BAI indexes.").ExistingFiles()
		sexPloidy = sexCmd.Flag("ploidy", "ploidy to use for copy number estimate.").Default("2").Uint()
	)

	app.HelpFlag.Short('h')
	app.VersionFlag.Short('v')

	cmd, err := app.Parse(os.Args[1:])

	var (
		idxs []string
		ok   bool
	)

	switch cmd {
	case "copy", "density", "sex":
		if *faiPath == "" {
			fmt.Fprintf(os.Stderr, "No reference FAI index provided!\n\n")
			app.Usage(os.Args[1:])
			os.Exit(1)
		}
		idxs, ok = readIndexes([3][]string{*copyIdxs, *densIdxs, *sexIdxs})
		if !ok {
			fmt.Fprintf(os.Stderr, "No indexes provided!\n\n")
			app.Usage(os.Args[1:])
			os.Exit(1)
		}
	}

	switch kingpin.MustParse(cmd, err) {
	case "copy":
		runCopy(idxs, *faiPath, *copyPloidy, *nProcs)
	case "density":
		runDensity(idxs, *faiPath, *densRaw, *nProcs)
	case "sex":
		runSex(idxs, *faiPath, *sexPloidy, *nProcs)
	case "help":
		fmt.Fprintln(os.Stderr, app.Help)
		os.Exit(1)
	}

}
