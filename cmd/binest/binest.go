package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/nygenome/binest"
)

var (
	buildTime    = ""
	gitCommit    = ""
	errNoIndexes = errors.New("no indexes provided to process")
)

type exitStatus int

type cli struct {
	Version kong.VersionFlag `short:"v" help:"Show application version."`
	FAI     string           `short:"f" name:"fai" type:"existingfile" help:"Path to reference FASTA index (*.fai). Compact FAI files are allowed for BAI-only workflows when no BAM header is available."`

	ChromCopy chromCopyCmd `cmd:"" name:"chromcopy" help:"Estimate per chromosome copy number for sample(s) given their indexes."`
	Size      sizeCmd      `cmd:"" name:"size" help:"Compute raw/normalized size for every 16kb window for sample(s) given their indexes."`
	Sex       sexCmd       `cmd:"" name:"sex" help:"Estimate sex genotype for sample(s) given their indexes."`
	Numreads  numreadsCmd  `cmd:"" name:"numreads" help:"Print the total number of reads for sample(s) given their BAM indexes."`
}

type chromCopyCmd struct {
	Ploidy              uint     `default:"2" help:"Base ploidy to use for chromosome copy estimate."`
	ReferenceBuild      string   `name:"reference-build" default:"auto" enum:"auto,b37,b38,none" help:"Reference build used for zero-bin masking: auto detects b37/b38 from primary and sex contig lengths; none disables masking."`
	AllowBAMFAIMismatch bool     `name:"allow-bam-fai-mismatch" help:"Warn and continue with --fai labels when a BAM header and FAI have different reference names, lengths, or order."`
	Indexes             []string `arg:"" optional:"" name:"index" type:"existingfile" help:"Path to one or more index files."`
}

type sizeCmd struct {
	Raw                 bool     `short:"r" help:"Output raw index-density estimates before autosomal-median scaling."`
	ReferenceBuild      string   `name:"reference-build" default:"auto" enum:"auto,b37,b38,none" help:"Reference build used for zero-bin masking: auto detects b37/b38 from primary and sex contig lengths; none disables masking."`
	AllowBAMFAIMismatch bool     `name:"allow-bam-fai-mismatch" help:"Warn and continue with --fai labels when a BAM header and FAI have different reference names, lengths, or order."`
	Indexes             []string `arg:"" optional:"" name:"index" type:"existingfile" help:"Path to one or more index files."`
}

type sexCmd struct {
	Ploidy              uint     `default:"2" help:"Base ploidy to use for sex genotype estimate."`
	ForceMaleFemale     bool     `name:"force-male-female" help:"Force male/female gender based on normalized value thresholds."`
	ReferenceBuild      string   `name:"reference-build" default:"auto" enum:"auto,b37,b38,none" help:"Reference build used for zero-bin masking: auto detects b37/b38 from primary and sex contig lengths; none disables masking."`
	AllowBAMFAIMismatch bool     `name:"allow-bam-fai-mismatch" help:"Warn and continue with --fai labels when a BAM header and FAI have different reference names, lengths, or order."`
	Indexes             []string `arg:"" optional:"" name:"index" type:"existingfile" help:"Path to one or more index files."`
}

type numreadsCmd struct {
	IncludeUnmapped bool     `name:"include-unmapped" help:"Include unmapped reads in count."`
	Indexes         []string `arg:"" optional:"" name:"index" type:"existingfile" help:"Path to one or more index files."`
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
	opts, err := referenceOptions(env.cli.FAI, c.ReferenceBuild, c.AllowBAMFAIMismatch)
	if err != nil {
		return err
	}
	return runChromCopy(c.Indexes, env, opts, c.Ploidy)
}

func (c *sizeCmd) Run(env *commandEnv) error {
	opts, err := referenceOptions(env.cli.FAI, c.ReferenceBuild, c.AllowBAMFAIMismatch)
	if err != nil {
		return err
	}
	return runSize(c.Indexes, env, opts, c.Raw)
}

func (c *sexCmd) Run(env *commandEnv) error {
	opts, err := referenceOptions(env.cli.FAI, c.ReferenceBuild, c.AllowBAMFAIMismatch)
	if err != nil {
		return err
	}
	return runSex(c.Indexes, env, opts, c.Ploidy, c.ForceMaleFemale)
}

func (c *numreadsCmd) Run(env *commandEnv) error {
	source, err := newCLIIndexSource(c.Indexes, env.stdin)
	if err != nil {
		return err
	}
	return binest.RunNumreads(source, env.stdout, c.IncludeUnmapped)
}

func referenceOptions(faiPath, buildValue string, allowMismatch bool) (binest.IndexOptions, error) {
	build, err := binest.ParseReferenceBuild(buildValue)
	if err != nil {
		return binest.IndexOptions{}, err
	}
	policy := binest.ReferenceValidationStrict
	if allowMismatch {
		policy = binest.ReferenceValidationAllowMismatch
	}
	return binest.IndexOptions{
		FAIPath:             faiPath,
		ReferenceBuild:      build,
		ReferenceValidation: policy,
	}, nil
}

func runSize(args []string, env *commandEnv, opts binest.IndexOptions, rawSize bool) error {
	source, err := newCLIIndexSource(args, env.stdin)
	if err != nil {
		return err
	}
	sizeString := "NORMALIZED_SIZE"
	if rawSize {
		sizeString = "RAW_SIZE"
	}
	if _, err := fmt.Fprintf(env.stdout, "CHROM\tSTART\tEND\t%s\tSAMPLE\n", sizeString); err != nil {
		return err
	}
	if err := flushCommandOutput(env.stdout); err != nil {
		return err
	}

	var batch binest.BatchError
	for {
		idxPath, ok, err := source.Next()
		if err != nil {
			batch.Add("", err)
			return batch.Err()
		}
		if !ok {
			return batch.Err()
		}
		idx, err := newIndexForCLI(idxPath, opts, env.stderr)
		if err != nil {
			batch.Add(idxPath, err)
			continue
		}
		sizes, err := idx.Sizes(rawSize)
		if err != nil {
			batch.Add(idxPath, err)
			continue
		}
		if _, err := fmt.Fprintln(env.stdout, sizes); err != nil {
			return err
		}
		if err := flushCommandOutput(env.stdout); err != nil {
			return err
		}
	}
}

func runChromCopy(args []string, env *commandEnv, opts binest.IndexOptions, ploidy uint) error {
	source, err := newCLIIndexSource(args, env.stdin)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(env.stdout, "SAMPLE\tCHROM\tCOPY_ESTIMATE\tNORM_ESTIMATE"); err != nil {
		return err
	}
	if err := flushCommandOutput(env.stdout); err != nil {
		return err
	}

	var batch binest.BatchError
	for {
		idxPath, ok, err := source.Next()
		if err != nil {
			batch.Add("", err)
			return batch.Err()
		}
		if !ok {
			return batch.Err()
		}
		idx, err := newIndexForCLI(idxPath, opts, env.stderr)
		if err != nil {
			batch.Add(idxPath, err)
			continue
		}
		copies, err := idx.ChromCopy(ploidy)
		if err != nil {
			batch.Add(idxPath, err)
			continue
		}
		if _, err := fmt.Fprintln(env.stdout, copies); err != nil {
			return err
		}
		if err := flushCommandOutput(env.stdout); err != nil {
			return err
		}
	}
}

func runSex(args []string, env *commandEnv, opts binest.IndexOptions, ploidy uint, forceMF bool) error {
	source, err := newCLIIndexSource(args, env.stdin)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(env.stdout, "SAMPLE\tESTIMATED_GENDER\tSEX_GENOTYPE\tNORM_X\tNORM_Y"); err != nil {
		return err
	}
	if err := flushCommandOutput(env.stdout); err != nil {
		return err
	}

	var batch binest.BatchError
	for {
		idxPath, ok, err := source.Next()
		if err != nil {
			batch.Add("", err)
			return batch.Err()
		}
		if !ok {
			return batch.Err()
		}
		idx, err := newIndexForCLI(idxPath, opts, env.stderr)
		if err != nil {
			batch.Add(idxPath, err)
			continue
		}
		sex, err := idx.Sex(ploidy, forceMF)
		if err != nil {
			batch.Add(idxPath, err)
			continue
		}
		if _, err := fmt.Fprintln(env.stdout, sex); err != nil {
			return err
		}
		if err := flushCommandOutput(env.stdout); err != nil {
			return err
		}
	}
}

func flushCommandOutput(w io.Writer) error {
	if flusher, ok := w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func newIndexForCLI(idxPath string, opts binest.IndexOptions, stderr io.Writer) (*binest.Index, error) {
	if opts.FAIPath != "" {
		report, err := binest.ValidateIndexReferences(idxPath, opts.FAIPath)
		if err != nil {
			return nil, err
		}
		if report.HasMismatch() {
			if opts.ReferenceValidation != binest.ReferenceValidationAllowMismatch {
				return nil, report
			}
			if _, err := fmt.Fprintln(stderr, report.Message(true)); err != nil {
				return nil, err
			}
		}
	}
	return binest.NewIndexWithOptions(idxPath, opts)
}

type cliIndexSource struct {
	args    []string
	nextArg int
	scanner *bufio.Scanner
}

func newCLIIndexSource(args []string, stdin io.Reader) (*cliIndexSource, error) {
	if len(args) == 0 && stdin == nil {
		return nil, errNoIndexes
	}
	source := &cliIndexSource{args: args}
	if stdin != nil {
		source.scanner = bufio.NewScanner(stdin)
		source.scanner.Buffer(make([]byte, 1024), 1024*1024)
	}
	return source, nil
}

func (s *cliIndexSource) Next() (string, bool, error) {
	if s.nextArg < len(s.args) {
		idxPath := s.args[s.nextArg]
		s.nextArg++
		return idxPath, true, nil
	}
	if s.scanner == nil {
		return "", false, nil
	}
	if s.scanner.Scan() {
		return s.scanner.Text(), true, nil
	}
	if err := s.scanner.Err(); err != nil {
		return "", false, fmt.Errorf("error reading data from stdin: %w", err)
	}
	return "", false, nil
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
