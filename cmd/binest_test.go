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
	for _, want := range []string{"Usage:", "size", "--raw"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout does not contain %q:\n%s", want, stdout.String())
		}
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
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

func TestParseDocumentedSizeFlags(t *testing.T) {
	faiPath := touchTempFile(t, "ref.fasta.fai")
	idxPath := touchTempFile(t, "sample.bai")
	app, err := parseTestCLI([]string{"size", "--fai", faiPath, "--raw", idxPath})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if app.FAI != faiPath {
		t.Fatalf("FAI = %q, want %q", app.FAI, faiPath)
	}
	if !app.Size.Raw {
		t.Fatal("Size.Raw = false, want true")
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

func TestRunIndexesStreamsArgsBeforeStdin(t *testing.T) {
	got, err := collectRunIndexes([]string{"arg.bai"}, strings.NewReader("stdin1.bai\nstdin2.bai\n"))
	if err != nil {
		t.Fatalf("runIndexes() error = %v", err)
	}

	want := []string{"arg.bai", "stdin1.bai", "stdin2.bai"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runIndexes() indexes = %#v, want %#v", got, want)
	}
}

func TestRunIndexesStreamsStdinBeforeEOF(t *testing.T) {
	stdin, stdinWriter := io.Pipe()
	processed := make(chan string, 1)
	runDone := make(chan error, 1)

	go func() {
		runDone <- runIndexes(nil, stdin, func(idxs <-chan string, errs chan<- error, done chan<- bool) {
			defer func() {
				done <- true
			}()
			for idxPath := range idxs {
				processed <- idxPath
			}
		})
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
		t.Fatal("runIndexes did not process stdin before EOF")
	}

	if err := stdinWriter.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("runIndexes() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runIndexes did not finish after stdin EOF")
	}
}

func TestRunIndexesNoInput(t *testing.T) {
	called := false
	err := runIndexes(nil, nil, func(<-chan string, chan<- error, chan<- bool) {
		called = true
	})
	if !errors.Is(err, errNoIndexes) {
		t.Fatalf("runIndexes() error = %v, want %v", err, errNoIndexes)
	}
	if called {
		t.Fatal("runIndexes called runner with no inputs")
	}
}

func collectRunIndexes(args []string, stdin io.Reader) ([]string, error) {
	var got []string
	err := runIndexes(args, stdin, func(idxs <-chan string, errs chan<- error, done chan<- bool) {
		defer func() {
			done <- true
		}()
		for idxPath := range idxs {
			got = append(got, idxPath)
		}
	})
	return got, err
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
