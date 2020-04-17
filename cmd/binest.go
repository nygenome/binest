package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"git.nygenome.org/rmusunuri/binest"
)

var (
	buildTime    = ""
	gitCommit    = ""
	errNoIndexes = errors.New("no indexes provided to process")
)

func main() {
	version := fmt.Sprintf(`binest %s
build time : %s
git commit : %s
`, binest.Version, buildTime, gitCommit)

	desc := "Estimate chromosome copy, sex and size from BAM/tabix index bins."
	app := kingpin.New("binest", desc).Version(version)

	// application flags
	fai := app.Flag("fai", "path to reference fasta index (*.fai).").Short('f').ExistingFile()

	// commands
	chrCpy := app.Command("chromcopy", "Estimate per chromosome copy number for sample(s) given their indexes.")
	size := app.Command("size", "Compute raw/normalized size for every 16kb window for sample(s) given their indexes.")
	sex := app.Command("sex", "Estimate sex genotype for sample(s) given their indexes.")

	// indexes
	cIdxs := chrCpy.Arg("index", "path to one or more index files.").ExistingFiles()
	zIdxs := size.Arg("index", "path to one or more index files.").ExistingFiles()
	xIdxs := sex.Arg("index", "path to one or more index files.").ExistingFiles()

	// other command flags
	cPloidy := chrCpy.Flag("ploidy", "base ploidy to use for chromosome copy estimate.").Default("2").Uint()
	zRaw := size.Flag("raw", "out raw sizes without normalization.").Short('r').Default("false").Bool()
	xPloidy := sex.Flag("ploidy", "base ploidy to use for sex genotype estimate.").Default("2").Uint()
	forceMF := sex.Flag("force-male-female", "Force male/female gender based on normalized value thresholds.").Default("false").Bool()

	app.HelpFlag.Short('h')
	app.VersionFlag.Short('v')

	cmd, err := app.Parse(os.Args[1:])
	if err != nil {
		app.Usage(os.Args[1:])
		os.Exit(1)
	}

	indexes := make(chan string, 100)
	errChan := make(chan error, 10)
	doneChan := make(chan bool, 1)

	fmt.Fprint(os.Stderr, version)

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	switch kingpin.MustParse(cmd, err) {
	case "chromcopy":
		go binest.RunChromCopy(indexes, errChan, doneChan, out, *fai, *cPloidy)
		go streamIndexes(*cIdxs, indexes, errChan)
	case "size":
		go binest.RunSize(indexes, errChan, doneChan, out, *fai, *zRaw)
		go streamIndexes(*zIdxs, indexes, errChan)
	case "sex":
		go binest.RunSex(indexes, errChan, doneChan, out, *fai, *xPloidy, *forceMF)
		go streamIndexes(*xIdxs, indexes, errChan)
	}

Loop:
	for {
		select {
		case err := <-errChan:
			if err == errNoIndexes {
				fmt.Fprintln(os.Stderr, "No indexes provided to process!")
				app.Usage(os.Args[1:])
				os.Exit(1)
			}
			panic(err)
		case <-doneChan:
			close(errChan)
			break Loop
		}
	}
}

func streamIndexes(args []string, idxsChan chan<- string, errChan chan<- error) {
	gotInput := len(args) > 0

	for _, arg := range args {
		idxsChan <- arg
	}

	if hasStdin() {
		scanner := bufio.NewScanner(os.Stdin)
		gotInput = true
		for scanner.Scan() {
			idxsChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "error reading data from stdin")
			errChan <- err
		}
	}

	if !gotInput {
		errChan <- errNoIndexes
	}

	close(idxsChan)
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
