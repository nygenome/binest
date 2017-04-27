package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/omicsnut/binest"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {

	var (
		desc    = "Estimate density, copy number and sex from BAI/TBI index bins"
		app     = kingpin.New("binest", desc).Version(fmt.Sprintf("binest v%s", binest.Version))
		faiPath = app.Flag("fai", "path to reference FAI index").ExistingFile()
		nProcs  = app.Flag("procs", "number of cores to use").Default("1").Uint()

		copyCmd    = app.Command("copy", "Estimate per chromosome copy number with BAI index")
		copyIdxs   = copyCmd.Arg("index", "path to one or more BAI indexes").ExistingFiles()
		copyPloidy = copyCmd.Flag("ploidy", "ploidy to use for copy number estimate").Default("2").Uint()

		densCmd  = app.Command("density", "Compute density across 16kb bins with BAI/TBI index")
		densIdxs = densCmd.Arg("index", "path to one or more BAI/TBI indexes").ExistingFiles()
		densRaw  = densCmd.Flag("raw", "output raw sizes without normalization").Default("false").Bool()

		sexCmd    = app.Command("sex", "Estimate sex genotype of a sample with BAI index")
		sexIdxs   = sexCmd.Arg("index", "path to one or more BAI indexes").ExistingFiles()
		sexPloidy = sexCmd.Flag("ploidy", "ploidy to use for copy number estimate").Default("2").Uint()
	)

	app.HelpFlag.Short('h')
	app.VersionFlag.Short('v')

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case "copy":
		if *faiPath == "" {
			kingpin.FatalUsage("No reference FAI index provided!")
		}

		indexes, ok := readIndexes(*copyIdxs)
		if !ok {
			kingpin.FatalUsage("No indexes provided to estimate copy number!")
		}

		runCopy(indexes, *faiPath, *copyPloidy, *nProcs)

	case "density":
		if *faiPath == "" {
			kingpin.FatalUsage("No reference FAI index provided!")
		}

		indexes, ok := readIndexes(*densIdxs)
		if !ok {
			kingpin.FatalUsage("No indexes provided to get densities!")
		}

		runDensity(indexes, *faiPath, *densRaw, *nProcs)

	case "sex":
		if *faiPath == "" {
			kingpin.FatalUsage("No reference FAI index provided!")
		}

		indexes, ok := readIndexes(*sexIdxs)
		if !ok {
			kingpin.FatalUsage("No indexes provided to estimate sex!")
		}

		runSex(indexes, *faiPath, *sexPloidy, *nProcs)

	case "help":
		fmt.Fprintln(os.Stderr, app.Help)
		os.Exit(1)
	}

}

// readIndexes merges all indexes if any, from both cli arguments and stdin
func readIndexes(args []string) ([]string, bool) {
	indexes := make([]string, 0, 100)
	indexes = append(indexes, args...)

	if hasStdin() {
		bufScanner := bufio.NewScanner(os.Stdin)
		for bufScanner.Scan() {
			indexes = append(indexes, bufScanner.Text())
		}
		if err := bufScanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "Error reading data from stdin")
			panic(err)
		}
	}

	return indexes, len(indexes) > 0
}

// hasStdin checks if process can read from /dev/stdin
func hasStdin() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error checking for data from stdin")
		panic(err)
	}
	if stat.Mode()&os.ModeCharDevice == 0 {
		return true
	}
	return false
}
