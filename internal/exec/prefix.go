// Package exec wraps os/exec for buildmux: prefixed line-by-line output
// forwarding, command construction for buildctl and manifest-tool.
package exec

import (
	"bytes"
	"io"
	"sync"
)

type PrefixWriter struct {
	mu     sync.Mutex
	dst    io.Writer
	prefix string
	buf    bytes.Buffer
}

func NewPrefixWriter(dst io.Writer, prefix string) *PrefixWriter {
	return &PrefixWriter{dst: dst, prefix: prefix}
}

func (w *PrefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadBytes('\n')
		if err != nil {
			// no complete line yet; stash remainder back
			w.buf.Reset()
			w.buf.Write(line)
			break
		}
		if _, err := io.WriteString(w.dst, w.prefix); err != nil {
			return 0, err
		}
		if _, err := w.dst.Write(line); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *PrefixWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() == 0 {
		return nil
	}
	if _, err := io.WriteString(w.dst, w.prefix); err != nil {
		return err
	}
	if _, err := w.dst.Write(w.buf.Bytes()); err != nil {
		return err
	}
	if !bytes.HasSuffix(w.buf.Bytes(), []byte{'\n'}) {
		if _, err := w.dst.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	w.buf.Reset()
	return nil
}
