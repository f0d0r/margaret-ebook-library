package util

import "io"

// ReadAtFull reads exactly len(buf) bytes at offset using random access. A
// short read reports io.ErrUnexpectedEOF; io.EOF alongside a full read is
// accepted, matching io.ReaderAt semantics.
func ReadAtFull(r io.ReaderAt, buf []byte, offset int64) error {
	n, err := r.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return err
	}
	if n != len(buf) {
		return io.ErrUnexpectedEOF
	}
	return nil
}
