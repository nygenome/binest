package binest

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/biogo/hts/bgzf"

	"git.nygenome.org/rmusunuri/binest/internal"
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

func TestMedianI64CurrentEvenLengthBehaviorProof(t *testing.T) {
	vals := []int64{10, 20, 30, 40}
	got := medianI64(vals)
	if got != 30 {
		t.Fatalf("medianI64([10 20 30 40]) = %v, want current characterized value 30", got)
	}

	mathematicallyCorrect := 25.0
	if got == mathematicallyCorrect {
		t.Fatalf("current median unexpectedly matches mathematically correct median %v", mathematicallyCorrect)
	}

	bins := Bins{{10, 20}, {30, 40}, {25}, {25}}
	refMap := &RefMap{0: "1", 1: "2", 2: "X", 3: "Y"}
	idx := &Index{Bins: &bins, RefMap: refMap, Sample: "median-probe"}
	gotTSV := idx.Sizes(false).String()

	currentRows := []string{
		"1\t0\t16384\t0.3333333333333333\tmedian-probe",
		"1\t16384\t32768\t0.6666666666666666\tmedian-probe",
		"2\t0\t16384\t1\tmedian-probe",
		"2\t16384\t32768\t1.3333333333333333\tmedian-probe",
	}
	for _, row := range currentRows {
		if !strings.Contains(gotTSV, row) {
			t.Fatalf("normalized TSV missing current-behavior row %q:\n%s", row, gotTSV)
		}
	}

	correctedRow := "1\t0\t16384\t0.4\tmedian-probe"
	if strings.Contains(gotTSV, correctedRow) {
		t.Fatalf("current TSV unexpectedly contains corrected median row %q:\n%s", correctedRow, gotTSV)
	}
}

func TestGenomeBuildDetection(t *testing.T) {
	tests := []struct {
		name string
		refs RefMap
		want string
	}{
		{name: "chr-prefixed primary refs", refs: RefMap{0: "chr1", 1: "chr2"}, want: "b38"},
		{name: "unprefixed primary refs", refs: RefMap{0: "1", 1: "2"}, want: "b37"},
		{name: "excluded chr-prefixed refs only", refs: RefMap{0: "chrM", 1: "chrUn_KI270442v1", 2: "chr1_KI270706v1_random"}, want: "b37"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := (&test.refs).GenomeBuild(); got != test.want {
				t.Fatalf("GenomeBuild() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGenomeBuildChrPrefixesUseB38ZeroMask(t *testing.T) {
	refMap := &RefMap{0: "chr1", 1: "chr2"}
	if got := refMap.GenomeBuild(); got != "b38" {
		t.Fatalf("GenomeBuild() = %q, want b38", got)
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

	forcedB37Bins := binSizes(cloneRefIndexes(refIdxs), "b37")
	forcedB37Idx := &Index{Bins: forcedB37Bins, RefMap: refMap, Sample: "chr-build-probe"}
	if got := forcedB37Idx.Sizes(true).String(); got != "" {
		t.Fatalf("forced b37 output = %q, want no rows", got)
	}

	detectedB38Bins := binSizes(cloneRefIndexes(refIdxs), refMap.GenomeBuild())
	detectedIdx := &Index{Bins: detectedB38Bins, RefMap: refMap, Sample: "chr-build-probe"}
	want := "chr2\t5013504\t5029888\t655360\tchr-build-probe"
	if got := detectedIdx.Sizes(true).String(); got != want {
		t.Fatalf("detected b38 output = %q, want %q", got, want)
	}
}

func TestSizesChromCopyAndSexCharacterization(t *testing.T) {
	bins := Bins{{10, 10, 10}, {20}, {5}, {7}, {100}}
	refMap := &RefMap{0: "1", 1: "X", 2: "Y", 3: "chrM", 4: "GL0001"}
	idx := &Index{Bins: &bins, RefMap: refMap, Sample: "synthetic"}

	raw := idx.Sizes(true)
	if !reflect.DeepEqual(raw.Chroms, []string{"1", "X", "Y"}) {
		t.Fatalf("raw.Chroms = %#v, want 1/X/Y only", raw.Chroms)
	}
	if !reflect.DeepEqual(raw.Starts[0], []uint64{0, 16384, 32768}) {
		t.Fatalf("raw.Starts[0] = %#v", raw.Starts[0])
	}

	norm := idx.Sizes(false)
	if got := norm.NormEsts[0][0]; got != 1 {
		t.Fatalf("autosome normalized value = %v, want 1", got)
	}
	if got := norm.NormEsts[1][0]; got != 2 {
		t.Fatalf("X normalized value = %v, want 2", got)
	}
	if got := norm.NormEsts[2][0]; got != 0.5 {
		t.Fatalf("Y normalized value = %v, want 0.5", got)
	}

	copy := idx.ChromCopy(2)
	if !reflect.DeepEqual(copy.CopyNums, []uint8{2, 4, 1}) {
		t.Fatalf("copy.CopyNums = %#v, want [2 4 1]", copy.CopyNums)
	}

	sex := idx.Sex(2, false)
	if sex.Gender != "unknown" || sex.Genotype != "XXXY" || sex.NormXEst != 4 || sex.NormYEst != 1 {
		t.Fatalf("Sex() = %#v, want unknown XXXY 4/1", sex)
	}
	forced := idx.Sex(2, true)
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
	idxs := make(chan string)
	errs := make(chan error, 1)
	done := make(chan bool, 1)
	wantErr := errors.New("write failed")

	go RunSize(idxs, errs, done, errWriter{err: wantErr}, "", true)
	close(idxs)
	<-done

	select {
	case err := <-errs:
		if !errors.Is(err, wantErr) {
			t.Fatalf("RunSize error = %v, want %v", err, wantErr)
		}
	default:
		t.Fatal("RunSize did not report writer error")
	}
}

func TestMeanI64EmptyCurrentBehavior(t *testing.T) {
	if got := meanI64(nil); !math.IsNaN(got) {
		t.Fatalf("meanI64(nil) = %v, want NaN under current behavior", got)
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
