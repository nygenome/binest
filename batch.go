package binest

import (
	"fmt"
	"strings"
)

// IndexSource streams index paths to command runners.
type IndexSource interface {
	Next() (path string, ok bool, err error)
}

// SliceIndexSource streams a fixed list of index paths.
type SliceIndexSource struct {
	paths []string
	next  int
}

// NewSliceIndexSource returns an IndexSource for a fixed list of index paths.
func NewSliceIndexSource(paths []string) *SliceIndexSource {
	return &SliceIndexSource{paths: append([]string(nil), paths...)}
}

func (s *SliceIndexSource) Next() (string, bool, error) {
	if s.next >= len(s.paths) {
		return "", false, nil
	}
	path := s.paths[s.next]
	s.next++
	return path, true, nil
}

// BatchError collects per-index failures while allowing later inputs to run.
type BatchError struct {
	Failures []IndexFailure
}

// IndexFailure is one per-index processing failure.
type IndexFailure struct {
	Path string
	Err  error
}

func (e *BatchError) Add(path string, err error) {
	if err == nil {
		return
	}
	e.Failures = append(e.Failures, IndexFailure{Path: path, Err: err})
}

func (e *BatchError) Err() error {
	if e == nil || len(e.Failures) == 0 {
		return nil
	}
	return e
}

func (e *BatchError) Error() string {
	if e == nil || len(e.Failures) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d index processing error(s):", len(e.Failures))
	for _, failure := range e.Failures {
		if failure.Path == "" {
			fmt.Fprintf(&b, "\n  %v", failure.Err)
			continue
		}
		fmt.Fprintf(&b, "\n  %s: %v", failure.Path, failure.Err)
	}
	return b.String()
}

func (e *BatchError) Unwrap() []error {
	if e == nil || len(e.Failures) == 0 {
		return nil
	}
	errs := make([]error, 0, len(e.Failures))
	for _, failure := range e.Failures {
		errs = append(errs, failure.Err)
	}
	return errs
}
