package binest

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/biogo/hts/bgzf"

	"github.com/nygenome/binest/internal"
)

func TestDetectIndexKind(t *testing.T) {
	tests := []struct {
		path string
		want IndexKind
	}{
		{path: "sample.bai", want: BaiIndex},
		{path: "sample.vcf.gz.tbi", want: TbiIndex},
		{path: "sample.bam", want: UnkIndex},
		{path: "sample.bai.tmp", want: UnkIndex},
	}

	for _, test := range tests {
		if got := DetectIndexKind(test.path); got != test.want {
			t.Fatalf("DetectIndexKind(%q) = %s, want %s", test.path, got, test.want)
		}
	}
}

func TestStripKnownSuffixes(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/data/HG001.final.bam.bai", want: "HG001"},
		{path: "/data/HG001.final.bai", want: "HG001"},
		{path: "/data/HG001.bam.bai", want: "HG001"},
		{path: "/data/HG001.bai", want: "HG001"},
		{path: "/data/HG001.vcf.gz.tbi", want: "HG001"},
		{path: "/data/HG001.cram.crai", want: "HG001.cram.crai"},
	}

	for _, test := range tests {
		if got := stripKnownSuffixes(test.path); got != test.want {
			t.Fatalf("stripKnownSuffixes(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}

func TestRoundChromSize(t *testing.T) {
	tests := []struct {
		norm float64
		want uint8
	}{
		{norm: 1.69, want: 1},
		{norm: 1.70, want: 2},
		{norm: 1.99, want: 2},
		{norm: 0.69, want: 0},
		{norm: 0.70, want: 1},
	}

	for _, test := range tests {
		if got := roundChromSize(test.norm); got != test.want {
			t.Fatalf("roundChromSize(%v) = %d, want %d", test.norm, got, test.want)
		}
	}
}

func TestMedianI64(t *testing.T) {
	tests := []struct {
		name string
		vals []int64
		want float64
	}{
		{name: "odd length", vals: []int64{30, 10, 20}, want: 20},
		{name: "even length", vals: []int64{10, 20, 30, 40}, want: 25},
		{name: "single value", vals: []int64{10}, want: 10},
		{name: "two values", vals: []int64{10, 20}, want: 15},
		{name: "zero median falls back to non-zero suffix", vals: []int64{0, 0, 0, 10, 20}, want: 15},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := medianI64(test.vals)
			if err != nil {
				t.Fatalf("medianI64(%v) error = %v", test.vals, err)
			}
			if got != test.want {
				t.Fatalf("medianI64(%v) = %v, want %v", test.vals, got, test.want)
			}
		})
	}
}

func TestMedianI64CorrectEvenLengthBehavior(t *testing.T) {
	vals := []int64{10, 20, 30, 40}
	got, err := medianI64(vals)
	if err != nil {
		t.Fatalf("medianI64([10 20 30 40]) error = %v", err)
	}
	if got != 25 {
		t.Fatalf("medianI64([10 20 30 40]) = %v, want 25", got)
	}

	bins := Bins{{10, 20}, {30, 40}, {25}, {25}}
	refMap := &RefMap{0: "1", 1: "2", 2: "X", 3: "Y"}
	idx := &Index{Bins: &bins, RefMap: refMap, Sample: "median-probe"}
	sizes, err := idx.Sizes(false)
	if err != nil {
		t.Fatalf("Sizes(false) error = %v", err)
	}
	gotTSV := sizes.String()

	correctedRows := []string{
		"1\t0\t16384\t0.4\tmedian-probe",
		"1\t16384\t32768\t0.8\tmedian-probe",
		"2\t0\t16384\t1.2\tmedian-probe",
		"2\t16384\t32768\t1.6\tmedian-probe",
	}
	for _, row := range correctedRows {
		if !strings.Contains(gotTSV, row) {
			t.Fatalf("normalized TSV missing corrected row %q:\n%s", row, gotTSV)
		}
	}
}

func TestReferenceBuildDetection(t *testing.T) {
	tests := []struct {
		name string
		refs referenceRecords
		want ReferenceBuild
	}{
		{name: "chr-prefixed b37 primary refs", refs: referenceRecords{{Name: "chr1", Length: b37Lengths["1"]}, {Name: "chr2", Length: b37Lengths["2"]}}, want: ReferenceBuildB37},
		{name: "unprefixed b38 primary refs", refs: referenceRecords{{Name: "1", Length: b38Lengths["1"]}, {Name: "2", Length: b38Lengths["2"]}}, want: ReferenceBuildB38},
		{name: "compact b38 reference", refs: referenceRecords{{Name: "chrX", Length: b38Lengths["X"]}}, want: ReferenceBuildB38},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := detectReferenceBuild(test.refs)
			if err != nil {
				t.Fatalf("detectReferenceBuild() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("detectReferenceBuild() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestReferenceBuildAutoUsesLengthsForZeroMask(t *testing.T) {
	refMap := &RefMap{0: "chr1", 1: "chr2"}
	records := referenceRecords{{ID: 0, Name: "chr1", Length: b38Lengths["1"]}, {ID: 1, Name: "chr2", Length: b38Lengths["2"]}}
	build, err := detectReferenceBuild(records)
	if err != nil {
		t.Fatalf("detectReferenceBuild() error = %v", err)
	}
	if build != ReferenceBuildB38 {
		t.Fatalf("detectReferenceBuild() = %q, want b38", build)
	}
	if !zeros["b37"][1][306] {
		t.Fatal("test setup expects b37 zero mask at ref=1 bin=306")
	}
	if zeros["b38"][1][306] {
		t.Fatal("test setup expects b38 not to zero mask ref=1 bin=306")
	}

	refIdxs := []internal.RefIndex{
		{},
		{Intervals: make([]bgzf.Offset, 308)},
	}
	for i := 0; i <= 306; i++ {
		refIdxs[1].Intervals[i] = bgzf.Offset{File: 100}
	}
	refIdxs[1].Intervals[307] = bgzf.Offset{File: 110}

	forcedB37Bins, err := binSizes(cloneRefIndexes(refIdxs), "b37")
	if err != nil {
		t.Fatalf("binSizes(b37) error = %v", err)
	}
	forcedB37Idx := &Index{Bins: forcedB37Bins, RefMap: refMap, Sample: "chr-build-probe"}
	forcedSizes, err := forcedB37Idx.Sizes(true)
	if err != nil {
		t.Fatalf("forced b37 Sizes(true) error = %v", err)
	}
	if got := forcedSizes.String(); got != "" {
		t.Fatalf("forced b37 output = %q, want no rows", got)
	}

	detectedB38Bins, err := binSizes(cloneRefIndexes(refIdxs), build.zeroMaskKey())
	if err != nil {
		t.Fatalf("binSizes(b38) error = %v", err)
	}
	detectedIdx := &Index{Bins: detectedB38Bins, RefMap: refMap, Sample: "chr-build-probe"}
	want := "chr2\t5013504\t5029888\t655360\tchr-build-probe"
	detectedSizes, err := detectedIdx.Sizes(true)
	if err != nil {
		t.Fatalf("detected b38 Sizes(true) error = %v", err)
	}
	if got := detectedSizes.String(); got != want {
		t.Fatalf("detected b38 output = %q, want %q", got, want)
	}
}

func TestSizesChromCopyAndSexCharacterization(t *testing.T) {
	bins := Bins{{10, 10, 10}, {20}, {5}, {7}, {100}}
	refMap := &RefMap{0: "1", 1: "X", 2: "Y", 3: "chrM", 4: "GL0001"}
	idx := &Index{Bins: &bins, RefMap: refMap, Sample: "synthetic"}

	raw, err := idx.Sizes(true)
	if err != nil {
		t.Fatalf("Sizes(true) error = %v", err)
	}
	if !reflect.DeepEqual(raw.Chroms, []string{"1", "X", "Y"}) {
		t.Fatalf("raw.Chroms = %#v, want 1/X/Y only", raw.Chroms)
	}
	if !reflect.DeepEqual(raw.Starts[0], []uint64{0, 16384, 32768}) {
		t.Fatalf("raw.Starts[0] = %#v", raw.Starts[0])
	}

	norm, err := idx.Sizes(false)
	if err != nil {
		t.Fatalf("Sizes(false) error = %v", err)
	}
	if got := norm.NormEsts[0][0]; got != 1 {
		t.Fatalf("autosome normalized value = %v, want 1", got)
	}
	if got := norm.NormEsts[1][0]; got != 2 {
		t.Fatalf("X normalized value = %v, want 2", got)
	}
	if got := norm.NormEsts[2][0]; got != 0.5 {
		t.Fatalf("Y normalized value = %v, want 0.5", got)
	}

	copy, err := idx.ChromCopy(2)
	if err != nil {
		t.Fatalf("ChromCopy() error = %v", err)
	}
	if !reflect.DeepEqual(copy.CopyNums, []uint8{2, 4, 1}) {
		t.Fatalf("copy.CopyNums = %#v, want [2 4 1]", copy.CopyNums)
	}

	sex, err := idx.Sex(2, false)
	if err != nil {
		t.Fatalf("Sex(false) error = %v", err)
	}
	if sex.Gender != "unknown" || sex.Genotype != "XXXY" || sex.NormXEst != 4 || sex.NormYEst != 1 {
		t.Fatalf("Sex() = %#v, want unknown XXXY 4/1", sex)
	}
	forced, err := idx.Sex(2, true)
	if err != nil {
		t.Fatalf("Sex(true) error = %v", err)
	}
	if forced.Gender != "male" {
		t.Fatalf("forced Sex().Gender = %q, want male", forced.Gender)
	}
}

func TestEstimateSexThresholds(t *testing.T) {
	tests := []struct {
		name     string
		xNorm    float64
		yNorm    float64
		forceMF  bool
		gender   string
		genotype string
	}{
		{name: "female", xNorm: 2, yNorm: 0, gender: "female", genotype: "XX"},
		{name: "male", xNorm: 1, yNorm: 1, gender: "male", genotype: "XY"},
		{name: "xo rescued to xx", xNorm: 1.6, yNorm: 0.1, gender: "female", genotype: "XX"},
		{name: "xo xy ambiguous called male", xNorm: 1, yNorm: 0.5, gender: "male", genotype: "XO/XY"},
		{name: "xo unknown", xNorm: 1, yNorm: 0, gender: "unknown", genotype: "XO"},
		{name: "force female", xNorm: 3, yNorm: 0, forceMF: true, gender: "female", genotype: "XXX"},
		{name: "force male", xNorm: 3, yNorm: 1, forceMF: true, gender: "male", genotype: "XXXY"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := estimateSex(test.xNorm, test.yNorm, test.forceMF)
			if got.Gender != test.gender || got.Genotype != test.genotype {
				t.Fatalf("estimateSex() = gender %q genotype %q, want %q %q",
					got.Gender, got.Genotype, test.gender, test.genotype)
			}
		})
	}
}

func TestResultStringFormatting(t *testing.T) {
	sizes := &Sizes{
		Sample:   "sample",
		Chroms:   []string{"1"},
		Starts:   [][]uint64{{0, 16384}},
		RawSizes: [][]int64{{10, 20}},
	}
	if got, want := sizes.String(), "1\t0\t16384\t10\tsample\n1\t16384\t32768\t20\tsample"; got != want {
		t.Fatalf("raw Sizes.String() = %q, want %q", got, want)
	}

	sizes.NormEsts = [][]float64{{1.25, 2.5}}
	if got, want := sizes.String(), "1\t0\t16384\t1.25\tsample\n1\t16384\t32768\t2.5\tsample"; got != want {
		t.Fatalf("normalized Sizes.String() = %q, want %q", got, want)
	}

	copy := &ChromCopy{Sample: "sample", Chroms: []string{"1"}, CopyNums: []uint8{2}, NormEsts: []float64{2}}
	if got, want := copy.String(), "sample\t1\t2\t2"; got != want {
		t.Fatalf("ChromCopy.String() = %q, want %q", got, want)
	}

	sex := &Sex{Sample: "sample", Gender: "male", Genotype: "XY", NormXEst: 1, NormYEst: 1}
	if got, want := sex.String(), "sample\tmale\tXY\t1\t1"; got != want {
		t.Fatalf("Sex.String() = %q, want %q", got, want)
	}
}

func TestRunSizePropagatesWriterError(t *testing.T) {
	wantErr := errors.New("write failed")

	err := RunSize(NewSliceIndexSource(nil), errWriter{err: wantErr}, IndexOptions{}, true)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunSize error = %v, want %v", err, wantErr)
	}
}

func TestMedianHelpersRejectEmptyInput(t *testing.T) {
	if _, err := meanI64(nil); !errors.Is(err, errInvalidMedianInput) {
		t.Fatalf("meanI64(nil) error = %v, want %v", err, errInvalidMedianInput)
	}
	if _, err := medianI64(nil); !errors.Is(err, errInvalidMedianInput) {
		t.Fatalf("medianI64(nil) error = %v, want %v", err, errInvalidMedianInput)
	}
}

func TestNormalizedOperationsRequireUsableAutosomes(t *testing.T) {
	bins := Bins{{20}, {10}}
	refMap := &RefMap{0: "X", 1: "Y"}
	idx := &Index{Bins: &bins, RefMap: refMap, Sample: "no-autosomes"}

	if _, err := idx.Sizes(false); !errors.Is(err, errInvalidAutosomeMedian) {
		t.Fatalf("Sizes(false) error = %v, want %v", err, errInvalidAutosomeMedian)
	}
	if _, err := idx.ChromCopy(2); !errors.Is(err, errInvalidAutosomeMedian) {
		t.Fatalf("ChromCopy() error = %v, want %v", err, errInvalidAutosomeMedian)
	}
	if _, err := idx.Sex(2, false); !errors.Is(err, errInvalidAutosomeMedian) {
		t.Fatalf("Sex() error = %v, want %v", err, errInvalidAutosomeMedian)
	}
}

func TestNormalizeRejectsZeroAutosomeMedian(t *testing.T) {
	sizes := &Sizes{
		Sample:   "zero-autosome",
		Chroms:   []string{"1", "X"},
		RawSizes: [][]int64{{0, 0}, {10}},
	}

	if err := sizes.Normalize(); !errors.Is(err, errInvalidAutosomeMedian) {
		t.Fatalf("Normalize() error = %v, want %v", err, errInvalidAutosomeMedian)
	}
	if sizes.NormEsts != nil {
		t.Fatalf("Normalize() mutated NormEsts on error: %#v", sizes.NormEsts)
	}
}

func TestNormalizeRejectsNegativeRawSizes(t *testing.T) {
	sizes := &Sizes{
		Sample:   "negative-raw",
		Chroms:   []string{"1"},
		RawSizes: [][]int64{{10, -1}},
	}

	if err := sizes.Normalize(); !errors.Is(err, errMalformedIndex) {
		t.Fatalf("Normalize() error = %v, want %v", err, errMalformedIndex)
	}
}

func TestEmptyChromosomeBinsProduceZeroDerivedEstimates(t *testing.T) {
	bins := Bins{{10, 20}, {}, {}}
	refMap := &RefMap{0: "1", 1: "X", 2: "Y"}
	idx := &Index{Bins: &bins, RefMap: refMap, Sample: "empty-sex-chroms"}

	copy, err := idx.ChromCopy(2)
	if err != nil {
		t.Fatalf("ChromCopy() error = %v", err)
	}
	if !reflect.DeepEqual(copy.CopyNums, []uint8{2, 0, 0}) {
		t.Fatalf("copy.CopyNums = %#v, want [2 0 0]", copy.CopyNums)
	}
	if !reflect.DeepEqual(copy.NormEsts, []float64{2, 0, 0}) {
		t.Fatalf("copy.NormEsts = %#v, want [2 0 0]", copy.NormEsts)
	}

	sex, err := idx.Sex(2, false)
	if err != nil {
		t.Fatalf("Sex() error = %v", err)
	}
	if sex.NormXEst != 0 || sex.NormYEst != 0 || sex.Gender != "unknown" {
		t.Fatalf("Sex() = %#v, want zero X/Y estimates and unknown gender", sex)
	}
}

func TestBinSizesRejectNonMonotonicIntervals(t *testing.T) {
	refIdxs := []internal.RefIndex{{
		Intervals: []bgzf.Offset{
			{File: 10},
			{File: 9},
		},
	}}

	if _, err := binSizes(refIdxs, "b38"); !errors.Is(err, errMalformedIndex) {
		t.Fatalf("binSizes() error = %v, want %v", err, errMalformedIndex)
	}
}

func TestSizesRejectNegativeRawBins(t *testing.T) {
	bins := Bins{{-1}}
	refMap := &RefMap{0: "1"}
	idx := &Index{Bins: &bins, RefMap: refMap, Sample: "negative-bin"}

	if _, err := idx.Sizes(true); !errors.Is(err, errMalformedIndex) {
		t.Fatalf("Sizes(true) error = %v, want %v", err, errMalformedIndex)
	}
}

type errWriter struct {
	err error
}

func (w errWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func cloneRefIndexes(in []internal.RefIndex) []internal.RefIndex {
	out := make([]internal.RefIndex, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Intervals = append([]bgzf.Offset(nil), in[i].Intervals...)
	}
	return out
}
