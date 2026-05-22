package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--version"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0", code)
	}
	if stdout.String() != versionString() {
		t.Fatalf("stdout = %q, want %q", stdout.String(), versionString())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"size", "--help"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0", code)
	}
	for _, want := range []string{
		"Usage:",
		"size",
		"--raw",
		"Output raw index-density estimates before",
		"autosomal-median scaling.",
		"--reference-build",
		"zero-bin masking",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout does not contain %q:\n%s", want, stdout.String())
		}
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRootParseErrorsPrintFullHelpToStderr(t *testing.T) {
	var helpStdout, helpStderr bytes.Buffer
	helpCode := run([]string{"--help"}, nil, &helpStdout, &helpStderr)
	if helpCode != 0 {
		t.Fatalf("run(--help) exit code = %d, want 0", helpCode)
	}
	if helpStderr.String() != "" {
		t.Fatalf("run(--help) stderr = %q, want empty", helpStderr.String())
	}
	rootHelp := helpStdout.String()
	if rootHelp == "" {
		t.Fatal("run(--help) stdout is empty")
	}

	tests := []struct {
		name       string
		args       []string
		wantStderr []string
	}{
		{
			name:       "no args",
			wantStderr: []string{"binest: error: expected one of"},
		},
		{
			name:       "unknown root argument",
			args:       []string{"notacommand"},
			wantStderr: []string{"binest: error: unexpected argument notacommand"},
		},
		{
			name:       "unknown root flag",
			args:       []string{"--bad"},
			wantStderr: []string{"binest: error: unknown flag --bad"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(test.args, nil, &stdout, &stderr)
			if code == 0 {
				t.Fatal("run() exit code = 0, want nonzero")
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), rootHelp) {
				t.Fatalf("stderr does not contain root help:\n%s", stderr.String())
			}
			for _, want := range test.wantStderr {
				if !strings.Contains(stderr.String(), want) {
					t.Fatalf("stderr does not contain %q:\n%s", want, stderr.String())
				}
			}
		})
	}
}

func TestRunSubcommandParseErrorKeepsShortUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"size", "--bad"}, nil, &stdout, &stderr)

	if code == 0 {
		t.Fatal("run() exit code = 0, want nonzero")
	}
	for _, want := range []string{"Usage: binest size", `Run "binest size --help" for more information.`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout does not contain %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "Commands:") {
		t.Fatalf("stdout contains full root help, want short usage:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "binest: error: unknown flag --bad") {
		t.Fatalf("stderr does not contain parse error:\n%s", stderr.String())
	}
}

func TestRunNoIndexes(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"size"}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{versionString(), "No indexes provided to process!", "Usage:"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr does not contain %q:\n%s", want, stderr.String())
		}
	}
}

func TestRunFlushesStdoutOnCommandError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"size"}, strings.NewReader("missing.bai\n"), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	want := "CHROM\tSTART\tEND\tNORMALIZED_SIZE\tSAMPLE\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want flushed header %q", stdout.String(), want)
	}
	for _, want := range []string{versionString(), "no bam/fai file provided", "missing.bai"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr does not contain %q:\n%s", want, stderr.String())
		}
	}
	if strings.Contains(stderr.String(), "panic:") {
		t.Fatalf("stderr contains panic stack:\n%s", stderr.String())
	}
}

func TestRunReportsCleanStdinIndexErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      io.Reader
		wantStdout string
		wantStderr []string
	}{
		{
			name:       "unsupported stdin index",
			args:       []string{"size"},
			stdin:      strings.NewReader("sample.txt\n"),
			wantStdout: "CHROM\tSTART\tEND\tNORMALIZED_SIZE\tSAMPLE\n",
			wantStderr: []string{"unknown/unsupported index", "sample.txt"},
		},
		{
			name:       "missing fai for tbi",
			args:       []string{"size"},
			stdin:      strings.NewReader(touchTempFile(t, "sample.tbi") + "\n"),
			wantStdout: "CHROM\tSTART\tEND\tNORMALIZED_SIZE\tSAMPLE\n",
			wantStderr: []string{"no fai file provided to build refmap for tabix index", "sample.tbi"},
		},
		{
			name:       "missing bam and fai for bai",
			args:       []string{"size"},
			stdin:      strings.NewReader(touchTempFile(t, "sample.bai") + "\n"),
			wantStdout: "CHROM\tSTART\tEND\tNORMALIZED_SIZE\tSAMPLE\n",
			wantStderr: []string{"no bam/fai file provided", "sample.bai"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(test.args, test.stdin, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("run() exit code = %d, want 1", code)
			}
			if stdout.String() != test.wantStdout {
				t.Fatalf("stdout = %q, want %q", stdout.String(), test.wantStdout)
			}
			if !strings.Contains(stderr.String(), versionString()) {
				t.Fatalf("stderr does not contain version:\n%s", stderr.String())
			}
			for _, want := range test.wantStderr {
				if !strings.Contains(stderr.String(), want) {
					t.Fatalf("stderr does not contain %q:\n%s", want, stderr.String())
				}
			}
			if strings.Contains(stderr.String(), "panic:") {
				t.Fatalf("stderr contains panic stack:\n%s", stderr.String())
			}
		})
	}
}

func TestParseDocumentedSizeFlags(t *testing.T) {
	faiPath := touchTempFile(t, "ref.fasta.fai")
	idxPath := touchTempFile(t, "sample.bai")
	app, err := parseTestCLI([]string{"size", "--fai", faiPath, "--raw", "--reference-build", "none", "--allow-bam-fai-mismatch", idxPath})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if app.FAI != faiPath {
		t.Fatalf("FAI = %q, want %q", app.FAI, faiPath)
	}
	if !app.Size.Raw {
		t.Fatal("Size.Raw = false, want true")
	}
	if app.Size.ReferenceBuild != "none" {
		t.Fatalf("Size.ReferenceBuild = %q, want none", app.Size.ReferenceBuild)
	}
	if !app.Size.AllowBAMFAIMismatch {
		t.Fatal("Size.AllowBAMFAIMismatch = false, want true")
	}
	if !reflect.DeepEqual(app.Size.Indexes, []string{idxPath}) {
		t.Fatalf("Size.Indexes = %#v, want %#v", app.Size.Indexes, []string{idxPath})
	}
}

func TestParseCommandSpecificFlags(t *testing.T) {
	idxPath := touchTempFile(t, "sample.bai")

	app, err := parseTestCLI([]string{"sex", "--ploidy", "4", "--force-male-female", idxPath})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if app.Sex.Ploidy != 4 {
		t.Fatalf("Sex.Ploidy = %d, want 4", app.Sex.Ploidy)
	}
	if !app.Sex.ForceMaleFemale {
		t.Fatal("Sex.ForceMaleFemale = false, want true")
	}
	if !reflect.DeepEqual(app.Sex.Indexes, []string{idxPath}) {
		t.Fatalf("Sex.Indexes = %#v, want %#v", app.Sex.Indexes, []string{idxPath})
	}

	app, err = parseTestCLI([]string{"numreads", "--include-unmapped", idxPath})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !app.Numreads.IncludeUnmapped {
		t.Fatal("Numreads.IncludeUnmapped = false, want true")
	}

	app, err = parseTestCLI([]string{"chromcopy", "--ploidy", "3", idxPath})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if app.ChromCopy.Ploidy != 3 {
		t.Fatalf("ChromCopy.Ploidy = %d, want 3", app.ChromCopy.Ploidy)
	}
}

func TestCLIIndexSourceStreamsArgsBeforeStdin(t *testing.T) {
	got, err := collectCLIIndexSource([]string{"arg.bai"}, strings.NewReader("stdin1.bai\nstdin2.bai\n"))
	if err != nil {
		t.Fatalf("collectCLIIndexSource() error = %v", err)
	}

	want := []string{"arg.bai", "stdin1.bai", "stdin2.bai"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("indexes = %#v, want %#v", got, want)
	}
}

func TestCLIIndexSourceStreamsStdinBeforeEOF(t *testing.T) {
	stdin, stdinWriter := io.Pipe()
	processed := make(chan string, 1)
	runDone := make(chan error, 1)

	go func() {
		source, err := newCLIIndexSource(nil, stdin)
		if err != nil {
			runDone <- err
			return
		}
		for {
			idxPath, ok, err := source.Next()
			if err != nil {
				runDone <- err
				return
			}
			if !ok {
				runDone <- nil
				return
			}
			processed <- idxPath
		}
	}()

	if _, err := io.WriteString(stdinWriter, "stdin1.bai\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}

	select {
	case got := <-processed:
		if got != "stdin1.bai" {
			t.Fatalf("processed index = %q, want stdin1.bai", got)
		}
	case <-time.After(time.Second):
		t.Fatal("source did not process stdin before EOF")
	}

	if err := stdinWriter.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("source error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("source did not finish after stdin EOF")
	}
}

func TestRunReportsAllBatchIndexFailures(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"size", "--reference-build", "none"}, strings.NewReader("bad1.txt\nbad2.txt\n"), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	for _, want := range []string{"2 index processing error(s):", "bad1.txt", "bad2.txt"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr does not contain %q:\n%s", want, stderr.String())
		}
	}
}

func TestCLIIndexSourceNoInput(t *testing.T) {
	_, err := newCLIIndexSource(nil, nil)
	if !errors.Is(err, errNoIndexes) {
		t.Fatalf("newCLIIndexSource() error = %v, want %v", err, errNoIndexes)
	}
}

func collectCLIIndexSource(args []string, stdin io.Reader) ([]string, error) {
	source, err := newCLIIndexSource(args, stdin)
	if err != nil {
		return nil, err
	}
	var got []string
	for {
		idxPath, ok, err := source.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return got, nil
		}
		got = append(got, idxPath)
	}
}

func parseTestCLI(args []string) (*cli, error) {
	app := &cli{}
	parser, err := newParser(app, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		return nil, err
	}
	_, err = parser.Parse(args)
	return app, err
}

func touchTempFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	return path
}
