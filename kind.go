package binest

import "path/filepath"

// IndexKind is the kind of index
type IndexKind uint8

// constants holding the known and unknown index types
const (
	UnkIndex IndexKind = iota
	BaiIndex
	TbiIndex
)

func (ik IndexKind) String() string {
	switch ik {
	case UnkIndex:
		return "UNK"
	case BaiIndex:
		return "BAI"
	case TbiIndex:
		return "TBI"
	}
	return "NAN"
}

// DetectIndexKind detects the IndexKind given the path to a binest supported index
func DetectIndexKind(idxPath string) IndexKind {
	switch filepath.Ext(idxPath) {
	case ".bai":
		return BaiIndex
	case ".tbi":
		return TbiIndex
	}
	return UnkIndex
}
