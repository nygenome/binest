package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

func TestCollectIndexesFromStdin(t *testing.T) {
	got, err := collectIndexes([]string{"arg.bai"}, strings.NewReader("stdin1.bai\nstdin2.bai\n"))
	if err != nil {
		t.Fatalf("collectIndexes() error = %v", err)
	}

	want := []string{"arg.bai", "stdin1.bai", "stdin2.bai"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectIndexes() = %#v, want %#v", got, want)
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
