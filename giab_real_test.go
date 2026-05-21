package binest

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type giabFixture struct {
	ID       string
	Sample   string
	Role     string
	Kind     string
	Filename string
	Bytes    int64
	SHA256   string
	URL      string
}

type giabGolden struct {
	Outputs  map[string]outputGolden   `json:"outputs"`
	ReadBins map[string]readBinsGolden `json:"read_bins"`
}

type outputGolden struct {
	SHA256 string   `json:"sha256"`
	Lines  int      `json:"lines"`
	Header string   `json:"header"`
	First  []string `json:"first"`
	Last   []string `json:"last"`
}

type readBinsGolden struct {
	Refs         int `json:"refs"`
	NonemptyRefs int `json:"nonempty_refs"`
	NonzeroBins  int `json:"nonzero_bins"`
}

func TestGIABFixtureCache(t *testing.T) {
	cache := giabCacheDir(t)
	for _, fixture := range loadGIABManifest(t) {
		path := filepath.Join(cache, fixture.Filename)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read GIAB fixture %s: %v", fixture.ID, err)
		}
		if int64(len(data)) != fixture.Bytes {
			t.Fatalf("%s size = %d, want %d", fixture.ID, len(data), fixture.Bytes)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != fixture.SHA256 {
			t.Fatalf("%s sha256 = %s, want %s", fixture.ID, got, fixture.SHA256)
		}
	}
}

func TestGIABReadBinsMetrics(t *testing.T) {
	cache := giabCacheDir(t)
	golden := loadGIABGolden(t)

	for filename, want := range golden.ReadBins {
		t.Run(filename, func(t *testing.T) {
			bins, err := ReadBins(filepath.Join(cache, filename), "b37")
			if err != nil {
				t.Fatalf("ReadBins(%q) error = %v", filename, err)
			}
			got := summarizeBins(bins)
			if got != want {
				t.Fatalf("ReadBins(%q) summary = %#v, want %#v", filename, got, want)
			}
		})
	}
}

func TestGIABNumreadsGoldens(t *testing.T) {
	cache := giabCacheDir(t)
	golden := loadGIABGolden(t)
	bais := giabPaths(cache, loadGIABManifest(t), "bai")

	assertOutputGolden(t, "numreads_mapped", runGIABNumreads(t, bais, false), golden.Outputs["numreads_mapped"])
	assertOutputGolden(t, "numreads_all", runGIABNumreads(t, bais, true), golden.Outputs["numreads_all"])
}

func TestGIABSizeRawGoldens(t *testing.T) {
	cache := giabCacheDir(t)
	golden := loadGIABGolden(t)
	manifest := loadGIABManifest(t)
	faiPath := filepath.Join("testdata", "giab", "grch38_1_22_xy_m.fai")

	assertOutputGolden(t, "size_bai_raw", runGIABSizeRaw(t, giabPaths(cache, manifest, "bai"), faiPath), golden.Outputs["size_bai_raw"])
	assertOutputGolden(t, "size_tbi_raw", runGIABSizeRaw(t, giabPaths(cache, manifest, "tbi"), faiPath), golden.Outputs["size_tbi_raw"])
}

func giabCacheDir(t *testing.T) string {
	t.Helper()
	if os.Getenv("BINEST_RUN_GIAB") != "1" {
		t.Skip("set BINEST_RUN_GIAB=1 or run make test-real to enable GIAB fixture tests")
	}
	cache := os.Getenv("BINEST_FIXTURE_CACHE")
	if cache == "" {
		cache = filepath.Join(".cache", "binest-fixtures", "giab")
	}
	return cache
}

func loadGIABManifest(t *testing.T) []giabFixture {
	t.Helper()

	fh, err := os.Open(filepath.Join("testdata", "giab", "manifest.tsv"))
	if err != nil {
		t.Fatalf("open GIAB manifest: %v", err)
	}
	defer func() {
		if err := fh.Close(); err != nil {
			t.Fatalf("close GIAB manifest: %v", err)
		}
	}()

	r := csv.NewReader(fh)
	r.Comma = '\t'
	r.FieldsPerRecord = -1

	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read GIAB manifest: %v", err)
	}
	if len(records) < 2 {
		t.Fatal("GIAB manifest has no fixture rows")
	}

	fixtures := make([]giabFixture, 0, len(records)-1)
	for _, record := range records[1:] {
		if len(record) < 8 {
			t.Fatalf("GIAB manifest row has %d fields, want at least 8: %#v", len(record), record)
		}
		bytes, err := strconv.ParseInt(record[5], 10, 64)
		if err != nil {
			t.Fatalf("parse bytes for %s: %v", record[0], err)
		}
		fixtures = append(fixtures, giabFixture{
			ID:       record[0],
			Sample:   record[1],
			Role:     record[2],
			Kind:     record[3],
			Filename: record[4],
			Bytes:    bytes,
			SHA256:   record[6],
			URL:      record[7],
		})
	}
	return fixtures
}

func loadGIABGolden(t *testing.T) giabGolden {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "golden", "giab_real.json"))
	if err != nil {
		t.Fatalf("read GIAB golden file: %v", err)
	}
	var golden giabGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("parse GIAB golden file: %v", err)
	}
	return golden
}

func giabPaths(cache string, fixtures []giabFixture, kind string) []string {
	paths := make([]string, 0, len(fixtures))
	for _, fixture := range fixtures {
		if fixture.Kind == kind {
			paths = append(paths, filepath.Join(cache, fixture.Filename))
		}
	}
	return paths
}

func runGIABNumreads(t *testing.T, paths []string, includeUnmapped bool) string {
	t.Helper()
	return runGIABCommand(t, paths, func(idxs <-chan string, errs chan<- error, done chan<- bool, out *bytes.Buffer) {
		RunNumreads(idxs, errs, done, out, includeUnmapped)
	})
}

func runGIABSizeRaw(t *testing.T, paths []string, faiPath string) string {
	t.Helper()
	return runGIABCommand(t, paths, func(idxs <-chan string, errs chan<- error, done chan<- bool, out *bytes.Buffer) {
		RunSize(idxs, errs, done, out, faiPath, true)
	})
}

func runGIABCommand(t *testing.T, paths []string, runner func(<-chan string, chan<- error, chan<- bool, *bytes.Buffer)) string {
	t.Helper()

	idxs := make(chan string, len(paths))
	errs := make(chan error, len(paths)+1)
	done := make(chan bool, 1)
	var out bytes.Buffer

	go runner(idxs, errs, done, &out)
	for _, path := range paths {
		idxs <- path
	}
	close(idxs)
	<-done

	select {
	case err := <-errs:
		if err != nil {
			t.Fatalf("GIAB command error = %v", err)
		}
	default:
	}
	return out.String()
}

func assertOutputGolden(t *testing.T, name string, got string, want outputGolden) {
	t.Helper()

	sum := sha256.Sum256([]byte(got))
	if gotSHA := hex.EncodeToString(sum[:]); gotSHA != want.SHA256 {
		t.Fatalf("%s sha256 = %s, want %s", name, gotSHA, want.SHA256)
	}

	lines := splitOutputLines(got)
	if len(lines) != want.Lines {
		t.Fatalf("%s line count = %d, want %d", name, len(lines), want.Lines)
	}
	if len(lines) == 0 || lines[0] != want.Header {
		t.Fatalf("%s header = %q, want %q", name, firstLine(lines), want.Header)
	}

	if gotFirst := prefix(lines, len(want.First)); !reflect.DeepEqual(gotFirst, want.First) {
		t.Fatalf("%s first lines = %#v, want %#v", name, gotFirst, want.First)
	}
	if gotLast := suffix(lines, len(want.Last)); !reflect.DeepEqual(gotLast, want.Last) {
		t.Fatalf("%s last lines = %#v, want %#v", name, gotLast, want.Last)
	}
}

func splitOutputLines(out string) []string {
	trimmed := strings.TrimSuffix(out, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func firstLine(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func prefix(lines []string, n int) []string {
	if len(lines) < n {
		return append([]string(nil), lines...)
	}
	return append([]string(nil), lines[:n]...)
}

func suffix(lines []string, n int) []string {
	if len(lines) < n {
		return append([]string(nil), lines...)
	}
	return append([]string(nil), lines[len(lines)-n:]...)
}

func summarizeBins(bins *Bins) readBinsGolden {
	var got readBinsGolden
	got.Refs = len(*bins)
	for _, ref := range *bins {
		if len(ref) > 0 {
			got.NonemptyRefs++
		}
		for _, size := range ref {
			if size != 0 {
				got.NonzeroBins++
			}
		}
	}
	return got
}

func (g readBinsGolden) String() string {
	return fmt.Sprintf("refs=%d nonempty_refs=%d nonzero_bins=%d", g.Refs, g.NonemptyRefs, g.NonzeroBins)
}
