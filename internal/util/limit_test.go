package util

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestLimitReaderExactSize(t *testing.T) {
	src := []byte("0123456789")
	lr := LimitReader(bytes.NewReader(src), int64(len(src)))

	data, err := io.ReadAll(lr)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if string(data) != string(src) {
		t.Errorf("data = %q, want %q", data, src)
	}
}

func TestLimitReaderUnderLimit(t *testing.T) {
	src := []byte("0123456789")
	lr := LimitReader(bytes.NewReader(src), 100)

	data, err := io.ReadAll(lr)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if string(data) != string(src) {
		t.Errorf("data = %q, want %q", data, src)
	}
}

func TestLimitReaderExceeded(t *testing.T) {
	src := bytes.Repeat([]byte{0xAB}, 32)
	lr := LimitReader(bytes.NewReader(src), 16)

	_, err := io.ReadAll(lr)
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("ReadAll() error = %v, want ErrLimitExceeded", err)
	}
}

func TestLimitReaderEmpty(t *testing.T) {
	lr := LimitReader(bytes.NewReader(nil), 16)

	data, err := io.ReadAll(lr)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("data length = %d, want 0", len(data))
	}
}

func TestLimitReaderZeroLimitNonEmpty(t *testing.T) {
	lr := LimitReader(bytes.NewReader([]byte("x")), 0)

	_, err := io.ReadAll(lr)
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("ReadAll() error = %v, want ErrLimitExceeded", err)
	}
}
