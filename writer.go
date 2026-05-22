package binest

import "io"

type flushWriter interface {
	Flush() error
}

func flushIfSupported(w io.Writer) error {
	if flusher, ok := w.(flushWriter); ok {
		return flusher.Flush()
	}
	return nil
}
