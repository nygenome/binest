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
	if handled, status := handleRootParseError(err, stderr); handled {
		return status
	}
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
	flushErr := out.Flush()
	if err == nil {
		err = flushErr
	} else if flushErr != nil {
		err = errors.Join(err, flushErr)
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
	if _, writeErr := fmt.Fprintln(stderr, err); writeErr != nil {
		return 1
	}
	return 1
}

func handleRootParseError(err error, stderr io.Writer) (bool, int) {
	var parseErr *kong.ParseError
	if !errors.As(err, &parseErr) || parseErr.Context == nil || parseErr.Context.Selected() != nil {
		return false, 0
	}
	if usageErr := printUsageTo(stderr, parseErr.Context); usageErr != nil {
		panic(usageErr)
	}
	if _, writeErr := fmt.Fprintln(stderr); writeErr != nil {
		return true, 1
	}
	if _, writeErr := fmt.Fprintf(stderr, "binest: error: %s\n", parseErr); writeErr != nil {
		return true, 1
	}
	return true, parseErr.ExitCode()
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
	return runIndexes(c.Indexes, env.stdin, func(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool) {
		binest.RunChromCopy(idxsChan, errChan, doneChan, env.stdout, env.cli.FAI, c.Ploidy)
	})
}

func (c *sizeCmd) Run(env *commandEnv) error {
	return runIndexes(c.Indexes, env.stdin, func(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool) {
		binest.RunSize(idxsChan, errChan, doneChan, env.stdout, env.cli.FAI, c.Raw)
	})
}

func (c *sexCmd) Run(env *commandEnv) error {
	return runIndexes(c.Indexes, env.stdin, func(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool) {
		binest.RunSex(idxsChan, errChan, doneChan, env.stdout, env.cli.FAI, c.Ploidy, c.ForceMaleFemale)
	})
}

func (c *numreadsCmd) Run(env *commandEnv) error {
	return runIndexes(c.Indexes, env.stdin, func(idxsChan <-chan string, errChan chan<- error, doneChan chan<- bool) {
		binest.RunNumreads(idxsChan, errChan, doneChan, env.stdout, c.IncludeUnmapped)
	})
}

func runIndexes(args []string, stdin io.Reader, runner func(<-chan string, chan<- error, chan<- bool)) error {
	if len(args) == 0 && stdin == nil {
		return errNoIndexes
	}

	idxsChan := make(chan string)
	errChan := make(chan error, 1)
	feedErrChan := make(chan error, 1)
	doneChan := make(chan bool, 1)
	stopChan := make(chan struct{})
	stopFeed := func() {
		select {
		case <-stopChan:
		default:
			close(stopChan)
		}
	}

	go runner(idxsChan, errChan, doneChan)

	go func() {
		defer close(idxsChan)

		for _, idxPath := range args {
			if !sendIndex(idxsChan, stopChan, idxPath) {
				return
			}
		}

		if stdin == nil {
			return
		}

		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			if !sendIndex(idxsChan, stopChan, scanner.Text()) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			feedErrChan <- fmt.Errorf("error reading data from stdin: %w", err)
		}
	}()

	var firstErr error
	for {
		select {
		case err := <-errChan:
			if err != nil && firstErr == nil {
				firstErr = err
			}
		case err := <-feedErrChan:
			if err != nil && firstErr == nil {
				firstErr = err
				stopFeed()
			}
		case <-doneChan:
			stopFeed()
			select {
			case err := <-errChan:
				if err != nil && firstErr == nil {
					firstErr = err
				}
			default:
			}
			select {
			case err := <-feedErrChan:
				if err != nil && firstErr == nil {
					firstErr = err
				}
			default:
			}
			return firstErr
		}
	}
}

func sendIndex(idxsChan chan<- string, stopChan <-chan struct{}, idxPath string) bool {
	select {
	case <-stopChan:
		return false
	case idxsChan <- idxPath:
		return true
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
