package util

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestReadAtFull(t *testing.T) {
	src := []byte("0123456789")

	t.Run("exact read", func(t *testing.T) {
		buf := make([]byte, 4)
		if err := ReadAtFull(bytes.NewReader(src), buf, 3); err != nil {
			t.Fatalf("ReadAtFull() error: %v", err)
		}
		if string(buf) != "3456" {
			t.Errorf("buf = %q, want %q", buf, "3456")
		}
	})

	t.Run("full read with EOF", func(t *testing.T) {
		// bytes.Reader returns io.EOF alongside a full final read.
		buf := make([]byte, 10)
		if err := ReadAtFull(bytes.NewReader(src), buf, 0); err != nil {
			t.Fatalf("ReadAtFull() error: %v", err)
		}
		if string(buf) != string(src) {
			t.Errorf("buf = %q, want %q", buf, src)
		}
	})

	t.Run("short read", func(t *testing.T) {
		buf := make([]byte, 4)
		if err := ReadAtFull(bytes.NewReader(src), buf, 8); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("ReadAtFull() error = %v, want ErrUnexpectedEOF", err)
		}
	})

	t.Run("reader error", func(t *testing.T) {
		want := errors.New("boom")
		r := errorReaderAt{err: want}
		buf := make([]byte, 4)
		if err := ReadAtFull(r, buf, 0); !errors.Is(err, want) {
			t.Errorf("ReadAtFull() error = %v, want %v", err, want)
		}
	})
}

type errorReaderAt struct {
	err error
}

func (r errorReaderAt) ReadAt([]byte, int64) (int, error) {
	return 0, r.err
}
