package exec

import (
	"bytes"
	"testing"
)

func TestPrefixWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewPrefixWriter(&buf, "[amd64] ")
	w.Write([]byte("hello\nworl"))
	w.Write([]byte("d\nfoo\n"))
	got := buf.String()
	want := "[amd64] hello\n[amd64] world\n[amd64] foo\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPrefixWriterFlushOnClose(t *testing.T) {
	var buf bytes.Buffer
	w := NewPrefixWriter(&buf, "[arm64] ")
	w.Write([]byte("no newline"))
	w.Close()
	got := buf.String()
	want := "[arm64] no newline\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
