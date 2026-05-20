package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"

	"git.nygenome.org/rmusunuri/binest"
)

var (
	buildTime    = ""
	gitCommit    = ""
	errNoIndexes = errors.New("no indexes provided to process")
)

type exitStatus int

type cli struct {
	Version kong.VersionFlag `short:"v" help:"Show application version."`
	FAI     string           `short:"f" name:"fai" type:"existingfile" help:"path to reference fasta index (*.fai)."`

	ChromCopy chromCopyCmd `cmd:"" name:"chromcopy" help:"Estimate per chromosome copy number for sample(s) given their indexes."`
	Size      sizeCmd      `cmd:"" name:"size" help:"Compute raw/normalized size for every 16kb window for sample(s) given their indexes."`
	Sex       sexCmd       `cmd:"" name:"sex" help:"Estimate sex genotype for sample(s) given their indexes."`
	Numreads  numreadsCmd  `cmd:"" name:"numreads" help:"Print the total number of reads for sample(s) given their BAM indexes."`
}

type chromCopyCmd struct {
	Ploidy  uint     `default:"2" help:"base ploidy to use for chromosome copy estimate."`
	Indexes []string `arg:"" optional:"" name:"index" type:"existingfile" help:"path to one or more index files."`
}

type sizeCmd struct {
	Raw     bool     `short:"r" help:"out raw sizes without normalization."`
	Indexes []string `arg:"" optional:"" name:"index" type:"existingfile" help:"path to one or more index files."`
}

type sexCmd struct {
	Ploidy          uint     `default:"2" help:"base ploidy to use for sex genotype estimate."`
	ForceMaleFemale bool     `name:"force-male-female" help:"Force male/female gender based on normalized value thresholds."`
	Indexes         []string `arg:"" optional:"" name:"index" type:"existingfile" help:"path to one or more index files."`
}

type numreadsCmd struct {
	IncludeUnmapped bool     `name:"include-unmapped" help:"Include unmapped reads in count."`
	Indexes         []string `arg:"" optional:"" name:"index" type:"existingfile" help:"path to one or more index files."`
}

type commandEnv struct {
	cli    *cli
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func main() {
	var stdin io.Reader
	if hasStdin(os.Stdin, os.Stderr) {
		stdin = os.Stdin
	}

	os.Exit(run(os.Args[1:], stdin, os.Stdout, os.Stderr))
}

func versionString() string {
	return fmt.Sprintf(`binest %s
build time : %s
git commit : %s
`, binest.Version, buildTime, gitCommit)
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (status int) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		if code, ok := recovered.(exitStatus); ok {
			status = int(code)
			return
		}
		panic(recovered)
	}()

	app := &cli{}
	parser, err := newParser(app, stdout, stderr)
	if err != nil {
		if _, writeErr := fmt.Fprintln(stderr, err); writeErr != nil {
			return 1
		}
		return 1
	}

	ctx, err := parser.Parse(args)
	parser.FatalIfErrorf(err)

	if _, err = fmt.Fprint(stderr, versionString()); err != nil {
		return 1
	}

	out := bufio.NewWriter(stdout)
	env := &commandEnv{
		cli:    app,
		stdin:  stdin,
		stdout: out,
		stderr: stderr,
	}

	err = ctx.Run(env)
	if flushErr := out.Flush(); err == nil {
		err = flushErr
	}
	if err == nil {
		return 0
	}
	if errors.Is(err, errNoIndexes) {
		if _, writeErr := fmt.Fprintln(stderr, "No indexes provided to process!"); writeErr != nil {
			return 1
		}
		if usageErr := printUsageTo(stderr, ctx); usageErr != nil {
			panic(usageErr)
		}
		return 1
	}
	panic(err)
}

func newParser(app *cli, stdout, stderr io.Writer) (*kong.Kong, error) {
	version := strings.TrimSuffix(versionString(), "\n")
	return kong.New(
		app,
		kong.Name("binest"),
		kong.Description("Estimate chromosome copy, sex and size from BAM/tabix index bins."),
		kong.Vars{"version": version},
		kong.Writers(stdout, stderr),
		kong.ShortUsageOnError(),
		kong.Exit(func(code int) {
			panic(exitStatus(code))
		}),
	)
}

func printUsageTo(w io.Writer, ctx *kong.Context) error {
	oldStdout := ctx.Stdout
	ctx.Stdout = w
	defer func() {
		ctx.Stdout = oldStdout
	}()
	return ctx.PrintUsage(false)
}

func (c *chromCopyCmd) Run(env *commandEnv) error {
	indexes, err := collectIndexes(c.Indexes, env.stdin)
	if err != nil {
		return err
	}
	return runIndexes(indexes, func(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool) {
		binest.RunChromCopy(idxsChan, errChan, doneChan, env.stdout, env.cli.FAI, c.Ploidy)
	})
}

func (c *sizeCmd) Run(env *commandEnv) error {
	indexes, err := collectIndexes(c.Indexes, env.stdin)
	if err != nil {
		return err
	}
	return runIndexes(indexes, func(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool) {
		binest.RunSize(idxsChan, errChan, doneChan, env.stdout, env.cli.FAI, c.Raw)
	})
}

func (c *sexCmd) Run(env *commandEnv) error {
	indexes, err := collectIndexes(c.Indexes, env.stdin)
	if err != nil {
		return err
	}
	return runIndexes(indexes, func(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool) {
		binest.RunSex(idxsChan, errChan, doneChan, env.stdout, env.cli.FAI, c.Ploidy, c.ForceMaleFemale)
	})
}

func (c *numreadsCmd) Run(env *commandEnv) error {
	indexes, err := collectIndexes(c.Indexes, env.stdin)
	if err != nil {
		return err
	}
	return runIndexes(indexes, func(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool) {
		binest.RunNumreads(idxsChan, errChan, doneChan, env.stdout, c.IncludeUnmapped)
	})
}

func collectIndexes(args []string, stdin io.Reader) ([]string, error) {
	indexes := append([]string(nil), args...)
	gotInput := len(indexes) > 0

	if stdin != nil {
		gotInput = true
		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			indexes = append(indexes, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading data from stdin: %w", err)
		}
	}

	if !gotInput {
		return nil, errNoIndexes
	}
	return indexes, nil
}

func runIndexes(indexes []string, runner func(<-chan string, chan<- error, chan<- bool)) error {
	idxsChan := make(chan string, len(indexes))
	errChan := make(chan error, len(indexes)+1)
	doneChan := make(chan bool, 1)

	go runner(idxsChan, errChan, doneChan)

	for _, idxPath := range indexes {
		idxsChan <- idxPath
	}
	close(idxsChan)

	for {
		select {
		case err := <-errChan:
			if err != nil {
				return err
			}
		case <-doneChan:
			select {
			case err := <-errChan:
				if err != nil {
					return err
				}
			default:
			}
			return nil
		}
	}
}

// hasStdin checks if process can read from stdin.
func hasStdin(stdin *os.File, stderr io.Writer) bool {
	stat, err := stdin.Stat()
	if err != nil {
		if _, writeErr := fmt.Fprintln(stderr, "Error checking for data from stdin"); writeErr != nil {
			panic(errors.Join(err, writeErr))
		}
		panic(err)
	}
	return stat.Mode()&os.ModeCharDevice == 0
}
