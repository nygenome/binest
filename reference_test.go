package binest

import (
	"errors"
	"strings"
	"testing"

	"github.com/biogo/hts/bam"
	"github.com/biogo/hts/bgzf"
	"github.com/biogo/hts/tabix"

	"github.com/nygenome/binest/internal"
)

func TestReferenceValidationReportsMismatches(t *testing.T) {
	tests := []struct {
		name       string
		index      referenceRecords
		fai        referenceRecords
		wantCounts map[string]int
		wantMatch  int
		wantUnused int
		wantAllow  int
	}{
		{
			name:       "exact match",
			index:      referenceRecords{{Name: "1", Length: 10}, {Name: "2", Length: 20}},
			fai:        referenceRecords{{Name: "1", Length: 10}, {Name: "2", Length: 20}},
			wantCounts: map[string]int{},
			wantMatch:  2,
		},
		{
			name:       "name mismatch",
			index:      referenceRecords{{Name: "1", Length: 10}},
			fai:        referenceRecords{{Name: "chr1", Length: 10}},
			wantCounts: map[string]int{"name": 1},
		},
		{
			name:       "length mismatch",
			index:      referenceRecords{{Name: "1", Length: 10}},
			fai:        referenceRecords{{Name: "1", Length: 11}},
			wantCounts: map[string]int{"length": 1},
		},
		{
			name:       "order mismatch",
			index:      referenceRecords{{Name: "1", Length: 10}, {Name: "2", Length: 20}},
			fai:        referenceRecords{{Name: "2", Length: 20}, {Name: "1", Length: 10}},
			wantCounts: map[string]int{"name": 2, "length": 2},
		},
		{
			name:       "compact fai allows excluded missing refs",
			index:      referenceRecords{{Name: "1", Length: 10}, {Name: "chrM", Length: 20}},
			fai:        referenceRecords{{Name: "1", Length: 10}},
			wantCounts: map[string]int{},
			wantMatch:  1,
			wantAllow:  1,
		},
		{
			name:       "compact fai rejects missing primary refs",
			index:      referenceRecords{{Name: "1", Length: 10}, {Name: "2", Length: 20}},
			fai:        referenceRecords{{Name: "1", Length: 10}},
			wantCounts: map[string]int{"missing": 1},
			wantMatch:  1,
		},
		{
			name:       "extra fai refs are reported as unused",
			index:      referenceRecords{{Name: "1", Length: 10}},
			fai:        referenceRecords{{Name: "1", Length: 10}, {Name: "2", Length: 20}},
			wantCounts: map[string]int{},
			wantMatch:  1,
			wantUnused: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := compareReferenceRecords("sample.bai", "ref.fa.fai", "BAM header", test.index, test.fai, true)
			if report.MatchedRecords != test.wantMatch {
				t.Fatalf("MatchedRecords = %d, want %d", report.MatchedRecords, test.wantMatch)
			}
			if report.UnusedFAI != test.wantUnused {
				t.Fatalf("UnusedFAI = %d, want %d", report.UnusedFAI, test.wantUnused)
			}
			if report.AllowedMissing != test.wantAllow {
				t.Fatalf("AllowedMissing = %d, want %d", report.AllowedMissing, test.wantAllow)
			}
			for _, kind := range []string{"name", "length", "missing", "extra-fai"} {
				if got := report.count(kind); got != test.wantCounts[kind] {
					t.Fatalf("count(%q) = %d, want %d; report = %s", kind, got, test.wantCounts[kind], report.Message(false))
				}
			}
		})
	}
}

func TestReferenceValidationWarningMessageIsActionable(t *testing.T) {
	report := compareReferenceRecords(
		"sample.bai",
		"ref.fa.fai",
		"BAM header",
		referenceRecords{{Name: "1", Length: 10}},
		referenceRecords{{Name: "chr1", Length: 10}},
		true,
	)

	message := report.Message(true)
	for _, want := range []string{
		"WARNING: BAM/FAI reference mismatch",
		"--allow-bam-fai-mismatch",
		"name mismatches: 1",
		"Likely cause:",
		"Next steps:",
		"Output coordinates and chromosome labels may be wrong.",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("warning message does not contain %q:\n%s", want, message)
		}
	}
}

func TestReferenceBuildDetectionRejectsUnknownAndMixedReferences(t *testing.T) {
	tests := []struct {
		name string
		refs referenceRecords
	}{
		{name: "no primary or sex lengths", refs: referenceRecords{{Name: "chrM", Length: 16569}}},
		{name: "unknown primary length", refs: referenceRecords{{Name: "1", Length: 12345}}},
		{name: "mixed b37 and b38 evidence", refs: referenceRecords{{Name: "1", Length: b37Lengths["1"]}, {Name: "2", Length: b38Lengths["2"]}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := detectReferenceBuild(test.refs); !errors.Is(err, errUnknownReferenceBuild) {
				t.Fatalf("detectReferenceBuild() error = %v, want %v", err, errUnknownReferenceBuild)
			}
		})
	}
}

func TestReferenceBuildNoneDisablesZeroMask(t *testing.T) {
	if !zeros["b37"][1][306] {
		t.Fatal("test setup expects b37 zero mask at ref=1 bin=306")
	}
	refIdxs := []internal.RefIndex{
		{},
		{Intervals: make([]bgzf.Offset, 308)},
	}
	for i := 0; i <= 306; i++ {
		refIdxs[1].Intervals[i] = bgzf.Offset{File: 100}
	}
	refIdxs[1].Intervals[307] = bgzf.Offset{File: 110}

	b37Bins, err := binSizes(cloneRefIndexes(refIdxs), ReferenceBuildB37.zeroMaskKey())
	if err != nil {
		t.Fatalf("binSizes(b37) error = %v", err)
	}
	if got := (*b37Bins)[1][306]; got != 0 {
		t.Fatalf("b37 zero-masked bin = %d, want 0", got)
	}

	noMaskBins, err := binSizes(cloneRefIndexes(refIdxs), ReferenceBuildNone.zeroMaskKey())
	if err != nil {
		t.Fatalf("binSizes(none) error = %v", err)
	}
	if got := (*noMaskBins)[1][306]; got == 0 {
		t.Fatalf("no-mask bin = %d, want non-zero", got)
	}
}

func TestBiogoIndexInternalsCanary(t *testing.T) {
	tests := map[string]any{
		"BAI": &bam.Index{},
		"TBI": &tabix.Index{},
	}
	for name, index := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := biogoRefIdxValue(index, name); err != nil {
				t.Fatalf("biogoRefIdxValue() error = %v", err)
			}
		})
	}
}
