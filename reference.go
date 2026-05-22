package binest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ReferenceBuild controls which build-specific zero-bin mask is applied.
type ReferenceBuild string

const (
	ReferenceBuildAuto ReferenceBuild = "auto"
	ReferenceBuildB37  ReferenceBuild = "b37"
	ReferenceBuildB38  ReferenceBuild = "b38"
	ReferenceBuildNone ReferenceBuild = "none"
)

var errUnknownReferenceBuild = errors.New("unknown or ambiguous reference build")

// ParseReferenceBuild parses a user-facing reference build value.
func ParseReferenceBuild(value string) (ReferenceBuild, error) {
	build := ReferenceBuild(strings.ToLower(strings.TrimSpace(value)))
	switch build {
	case "", ReferenceBuildAuto:
		return ReferenceBuildAuto, nil
	case ReferenceBuildB37, ReferenceBuildB38, ReferenceBuildNone:
		return build, nil
	default:
		return "", fmt.Errorf("unsupported reference build %q: expected auto, b37, b38, or none", value)
	}
}

func (b ReferenceBuild) String() string {
	if b == "" {
		return string(ReferenceBuildAuto)
	}
	return string(b)
}

func (b ReferenceBuild) zeroMaskKey() string {
	if b == ReferenceBuildNone {
		return ""
	}
	return string(b)
}

// ReferenceValidationPolicy controls how BAM/FAI compatibility checks are handled.
type ReferenceValidationPolicy uint8

const (
	ReferenceValidationStrict ReferenceValidationPolicy = iota
	ReferenceValidationAllowMismatch
)

// IndexOptions controls index construction behavior.
type IndexOptions struct {
	FAIPath             string
	ReferenceBuild      ReferenceBuild
	ReferenceValidation ReferenceValidationPolicy
}

func (o IndexOptions) withDefaults() IndexOptions {
	if o.ReferenceBuild == "" {
		o.ReferenceBuild = ReferenceBuildAuto
	}
	return o
}

type referenceRecord struct {
	ID     int
	Name   string
	Length int
}

type referenceRecords []referenceRecord

func (records referenceRecords) refMap() *RefMap {
	refMap := make(RefMap, len(records))
	for _, record := range records {
		refMap[record.ID] = record.Name
	}
	return &refMap
}

func (records referenceRecords) refLengths() *RefLengths {
	lengths := make(RefLengths, len(records))
	for _, record := range records {
		if record.Length > 0 {
			lengths[record.ID] = record.Length
		}
	}
	return &lengths
}

// ReferenceValidationReport describes compatibility between an index's reference
// names and a supplied FAI file.
type ReferenceValidationReport struct {
	IndexPath string
	FAIPath   string
	Source    string
	Issues    []ReferenceValidationIssue

	ComparedRecords int
	MatchedRecords  int
	IndexRecords    int
	FAIRecords      int
	AllowedMissing  int
	UnusedFAI       int
}

// ReferenceValidationIssue is one categorized reference compatibility issue.
type ReferenceValidationIssue struct {
	Kind        string
	RefID       int
	IndexName   string
	IndexLength int
	FAIName     string
	FAILength   int
}

func (r *ReferenceValidationReport) HasMismatch() bool {
	return r != nil && len(r.Issues) > 0
}

func (r *ReferenceValidationReport) Error() string {
	return r.Message(false)
}

// Message formats the report as an actionable error or warning.
func (r *ReferenceValidationReport) Message(warning bool) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	if warning {
		fmt.Fprintf(&b, "WARNING: BAM/FAI reference mismatch for %s\n", r.IndexPath)
		fmt.Fprintf(&b, "continuing with FAI labels because --allow-bam-fai-mismatch was set\n\n")
	} else {
		fmt.Fprintf(&b, "BAM/FAI reference mismatch for %s\n\n", r.IndexPath)
	}
	fmt.Fprintf(&b, "binest compared the %s references with --fai before using FAI labels for output.\n", r.Source)
	fmt.Fprintf(&b, "Summary:\n")
	fmt.Fprintf(&b, "  matched records: %d/%d\n", r.MatchedRecords, r.ComparedRecords)
	for _, kind := range []string{"name", "length", "missing", "extra-fai"} {
		if count := r.count(kind); count > 0 {
			fmt.Fprintf(&b, "  %s mismatches: %d\n", kind, count)
		}
	}
	if r.AllowedMissing > 0 {
		fmt.Fprintf(&b, "  extra index refs absent from compact FAI and ignored as excluded refs: %d\n", r.AllowedMissing)
	}
	if r.UnusedFAI > 0 {
		fmt.Fprintf(&b, "  trailing FAI refs unused by index: %d\n", r.UnusedFAI)
	}
	for _, kind := range []string{"name", "length", "missing", "extra-fai"} {
		if issue, ok := r.example(kind); ok {
			fmt.Fprintf(&b, "\nExample %s issue:\n", kind)
			fmt.Fprintf(&b, "  ref %d: index %q length %s, FAI %q length %s\n",
				issue.RefID, issue.IndexName, formatLength(issue.IndexLength),
				issue.FAIName, formatLength(issue.FAILength))
		}
	}
	if cause := r.likelyCause(); cause != "" {
		fmt.Fprintf(&b, "\nLikely cause:\n  %s\n", cause)
	}
	fmt.Fprintf(&b, "\nNext steps:\n")
	fmt.Fprintf(&b, "  provide an FAI generated from the same reference, omit --fai when a BAM header is available,\n")
	fmt.Fprintf(&b, "  or pass --allow-bam-fai-mismatch to continue with explicit FAI labels.\n")
	if warning {
		fmt.Fprintf(&b, "\nOutput coordinates and chromosome labels may be wrong.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (r *ReferenceValidationReport) count(kind string) int {
	count := 0
	for _, issue := range r.Issues {
		if issue.Kind == kind {
			count++
		}
	}
	return count
}

func (r *ReferenceValidationReport) example(kind string) (ReferenceValidationIssue, bool) {
	for _, issue := range r.Issues {
		if issue.Kind == kind {
			return issue, true
		}
	}
	return ReferenceValidationIssue{}, false
}

func (r *ReferenceValidationReport) likelyCause() string {
	if r.count("name") > 0 && r.count("length") == 0 {
		return `chromosome naming style differs, for example "1" vs "chr1".`
	}
	if r.count("length") > 0 {
		return "reference build, contig order, or FAI file differs from the indexed file."
	}
	if r.count("missing") > 0 {
		return "the FAI is missing references used by the index."
	}
	return ""
}

func formatLength(length int) string {
	if length <= 0 {
		return "unknown"
	}
	return strconv.Itoa(length)
}

func compareReferenceRecords(indexPath, faiPath, source string, indexRecords, faiRecords referenceRecords, compareLengths bool) *ReferenceValidationReport {
	report := &ReferenceValidationReport{
		IndexPath:    indexPath,
		FAIPath:      faiPath,
		Source:       source,
		IndexRecords: len(indexRecords),
		FAIRecords:   len(faiRecords),
	}

	limit := len(indexRecords)
	if len(faiRecords) < limit {
		limit = len(faiRecords)
	}
	report.ComparedRecords = limit
	for idx := 0; idx < limit; idx++ {
		indexRecord := indexRecords[idx]
		faiRecord := faiRecords[idx]
		matched := true
		if indexRecord.Name != faiRecord.Name {
			report.Issues = append(report.Issues, ReferenceValidationIssue{
				Kind:        "name",
				RefID:       idx,
				IndexName:   indexRecord.Name,
				IndexLength: indexRecord.Length,
				FAIName:     faiRecord.Name,
				FAILength:   faiRecord.Length,
			})
			matched = false
		}
		if compareLengths && indexRecord.Length > 0 && faiRecord.Length > 0 && indexRecord.Length != faiRecord.Length {
			report.Issues = append(report.Issues, ReferenceValidationIssue{
				Kind:        "length",
				RefID:       idx,
				IndexName:   indexRecord.Name,
				IndexLength: indexRecord.Length,
				FAIName:     faiRecord.Name,
				FAILength:   faiRecord.Length,
			})
			matched = false
		}
		if matched {
			report.MatchedRecords++
		}
	}

	for idx := limit; idx < len(indexRecords); idx++ {
		indexRecord := indexRecords[idx]
		if excludeChroms.MatchString(indexRecord.Name) {
			report.AllowedMissing++
			continue
		}
		report.Issues = append(report.Issues, ReferenceValidationIssue{
			Kind:        "missing",
			RefID:       idx,
			IndexName:   indexRecord.Name,
			IndexLength: indexRecord.Length,
		})
	}
	if len(faiRecords) > len(indexRecords) {
		report.UnusedFAI = len(faiRecords) - len(indexRecords)
	}
	return report
}

func detectReferenceBuild(records referenceRecords) (ReferenceBuild, error) {
	if len(records) == 0 {
		return "", fmt.Errorf("%w: no reference records with lengths", errUnknownReferenceBuild)
	}
	evidence := 0
	matchesB37 := true
	matchesB38 := true
	for _, record := range records {
		key, ok := canonicalBuildContig(record.Name)
		if !ok || record.Length <= 0 {
			continue
		}
		evidence++
		if b37Lengths[key] != record.Length {
			matchesB37 = false
		}
		if b38Lengths[key] != record.Length {
			matchesB38 = false
		}
	}
	if evidence == 0 {
		return "", fmt.Errorf("%w: no primary or sex contig lengths found", errUnknownReferenceBuild)
	}
	switch {
	case matchesB37 && !matchesB38:
		return ReferenceBuildB37, nil
	case matchesB38 && !matchesB37:
		return ReferenceBuildB38, nil
	default:
		return "", fmt.Errorf("%w: reference lengths do not uniquely match b37 or b38", errUnknownReferenceBuild)
	}
}

func canonicalBuildContig(name string) (string, bool) {
	name = strings.TrimPrefix(name, "chr")
	if name == "M" || name == "MT" {
		return "", false
	}
	if name == "X" || name == "Y" {
		return name, true
	}
	if n, err := strconv.Atoi(name); err == nil && n >= 1 && n <= 22 {
		return strconv.Itoa(n), true
	}
	return "", false
}

var b37Lengths = map[string]int{
	"1": 249250621, "2": 243199373, "3": 198022430, "4": 191154276,
	"5": 180915260, "6": 171115067, "7": 159138663, "8": 146364022,
	"9": 141213431, "10": 135534747, "11": 135006516, "12": 133851895,
	"13": 115169878, "14": 107349540, "15": 102531392, "16": 90354753,
	"17": 81195210, "18": 78077248, "19": 59128983, "20": 63025520,
	"21": 48129895, "22": 51304566, "X": 155270560, "Y": 59373566,
}

var b38Lengths = map[string]int{
	"1": 248956422, "2": 242193529, "3": 198295559, "4": 190214555,
	"5": 181538259, "6": 170805979, "7": 159345973, "8": 145138636,
	"9": 138394717, "10": 133797422, "11": 135086622, "12": 133275309,
	"13": 114364328, "14": 107043718, "15": 101991189, "16": 90338345,
	"17": 83257441, "18": 80373285, "19": 58617616, "20": 64444167,
	"21": 46709983, "22": 50818468, "X": 156040895, "Y": 57227415,
}
